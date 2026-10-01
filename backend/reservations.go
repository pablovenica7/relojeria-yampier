package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Estados de una reserva. Única definición en el backend: el resto del
// código usa estas constantes, nunca strings sueltos.
const (
	ReservationPending     = "pending"      // pedido registrado, sin unidad retenida
	ReservationConfirmed   = "confirmed"    // el local confirmó: retiene 1 unidad
	ReservationDepositPaid = "deposit_paid" // seña recibida: sigue retenida
	ReservationCompleted   = "completed"    // pagado y retirado en el local (venta)
	ReservationCancelled   = "cancelled"
	ReservationExpired     = "expired"
)

// reservationTransitions define las transiciones permitidas. Los estados
// finales (completed, cancelled, expired) no tienen salida: una reserva
// cerrada no se reabre; si el cliente vuelve, se crea una reserva nueva.
var reservationTransitions = map[string][]string{
	ReservationPending:     {ReservationConfirmed, ReservationCancelled, ReservationExpired},
	ReservationConfirmed:   {ReservationDepositPaid, ReservationCompleted, ReservationCancelled, ReservationExpired},
	ReservationDepositPaid: {ReservationCompleted, ReservationCancelled},
}

func validReservationStatus(s string) bool {
	switch s {
	case ReservationPending, ReservationConfirmed, ReservationDepositPaid,
		ReservationCompleted, ReservationCancelled, ReservationExpired:
		return true
	}
	return false
}

func canTransition(from, to string) bool {
	for _, s := range reservationTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// holdsStock indica si una reserva en ese estado debe tener una unidad retenida.
func holdsStock(status string) bool {
	return status == ReservationConfirmed || status == ReservationDepositPaid
}

// expirable: solo las reservas pendientes o confirmadas vencen solas. Una
// reserva con seña paga no se cancela automáticamente: lo decide el local.
func expirable(status string) bool {
	return status == ReservationPending || status == ReservationConfirmed
}

const (
	maxReservationDays = 90
	reservationNotes   = 2000
)

func validateReservationInput(in *ReservationInput, now time.Time) string {
	in.CustomerName = collapseSpaces(in.CustomerName)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Email = strings.TrimSpace(in.Email)
	in.Notes = strings.TrimSpace(in.Notes)
	in.WatchID = strings.TrimSpace(in.WatchID)
	in.InquiryID = strings.TrimSpace(in.InquiryID)
	if in.Status == "" {
		in.Status = ReservationPending
	}

	switch {
	case !primitive.IsValidObjectID(in.WatchID):
		return "Elegí un reloj válido"
	case in.InquiryID != "" && !primitive.IsValidObjectID(in.InquiryID):
		return "Consulta inválida"
	case in.CustomerName == "":
		return "El nombre del cliente es obligatorio"
	case len(in.CustomerName) > 120:
		return "El nombre del cliente es demasiado largo"
	case in.Phone == "" && in.Email == "":
		return "Indicá al menos un teléfono o un email del cliente"
	case len(in.Phone) > 40:
		return "El teléfono es demasiado largo"
	case in.Email != "" && (!emailRe.MatchString(in.Email) || len(in.Email) > 190):
		return "El email no es válido"
	case len(in.Notes) > reservationNotes:
		return "Las notas son demasiado largas (máximo 2000 caracteres)"
	case in.Status != ReservationPending && in.Status != ReservationConfirmed:
		return "Una reserva nueva solo puede crearse como pendiente o confirmada"
	case in.DepositAmount < 0 || in.DepositAmount > maxPriceARS:
		return "La seña es inválida"
	case in.PriceAtReservation < 0 || in.PriceAtReservation > maxPriceARS:
		return "El precio acordado es inválido"
	}
	if msg := validateExpiry(in.ExpiresAt, now); msg != "" {
		return msg
	}
	return ""
}

func validateExpiry(t *time.Time, now time.Time) string {
	if t == nil {
		return ""
	}
	if !t.After(now) {
		return "El vencimiento debe ser una fecha futura"
	}
	if t.After(now.AddDate(0, 0, maxReservationDays)) {
		return "El vencimiento no puede superar los 90 días"
	}
	return ""
}

// ---------- Stock: operaciones atómicas ----------
//
// MongoDB corre como instancia única (sin replica set), así que no hay
// transacciones multi-documento. La consistencia se logra así:
//
//  1. Retener una unidad es UNA operación condicional:
//     updateOne({_id, archivedAt: null, stockQuantity: {$gte: 1}}, {$inc: -1})
//     Mongo la aplica atómicamente: si dos reservas compiten por la última
//     unidad, solo una encuentra stockQuantity >= 1; la otra recibe 409.
//  2. El cambio de estado de la reserva también es condicional
//     ({_id, status: <estado leído>, stockHeld: <valor leído>}), así dos
//     operaciones simultáneas sobre la misma reserva no pueden aplicarse ambas
//     (por ejemplo, dos cancelaciones no devuelven dos unidades).
//  3. Orden elegido: al RETENER, primero se descuenta stock y después se marca
//     la reserva; al LIBERAR, primero se marca la reserva y después se devuelve
//     la unidad. Si el proceso se cae entre ambos pasos, el peor caso es una
//     unidad "perdida" (stock por debajo del real, corregible desde el Admin),
//     nunca una unidad vendida/reservada dos veces.

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// holdUnit descuenta una unidad de forma atómica y devuelve el reloj
// después del cambio (para registrar el movimiento con stock antes/después).
func holdUnit(ctx context.Context, watchID primitive.ObjectID) (*Watch, error) {
	var after Watch
	err := watchesCol().FindOneAndUpdate(ctx,
		bson.M{"_id": watchID, "archivedAt": nil, "stockQuantity": bson.M{"$gte": 1}},
		bson.M{"$inc": bson.M{"stockQuantity": -1}, "$set": bson.M{"updatedAt": time.Now()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&after)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, errCode(http.StatusConflict, CodeOutOfStock, "No hay stock disponible para este reloj")
	}
	if err != nil {
		return nil, err
	}
	return &after, nil
}

// releaseUnit devuelve una unidad. Devuelve nil si falló (queda en el log).
func releaseUnit(ctx context.Context, watchID primitive.ObjectID) *Watch {
	var after Watch
	err := watchesCol().FindOneAndUpdate(ctx, bson.M{"_id": watchID},
		bson.M{"$inc": bson.M{"stockQuantity": 1}, "$set": bson.M{"updatedAt": time.Now()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&after)
	if err != nil {
		// La reserva ya quedó cancelada/vencida; la unidad no volvió al stock.
		// Se registra para corregirla a mano desde el Admin.
		log.Printf("ATENCIÓN: no se pudo devolver 1 unidad al stock del reloj %s: %v", watchID.Hex(), err)
		return nil
	}
	return &after
}

// releaseAndRecord devuelve la unidad de una reserva y anota el movimiento.
func releaseAndRecord(ctx context.Context, r *Reservation, reason, actor string) {
	if after := releaseUnit(ctx, r.WatchID); after != nil {
		recordMovement(ctx, movementFromWatch(after, MoveReservationRelease, 1, &r.ID, reason, actor))
	}
}

func findReservation(ctx context.Context, id primitive.ObjectID) (*Reservation, error) {
	var r Reservation
	if err := reservationsCol().FindOne(ctx, bson.M{"_id": id}).Decode(&r); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errCode(http.StatusNotFound, CodeReservationNotFound, "Reserva no encontrada")
		}
		return nil, err
	}
	return &r, nil
}

// createReservation registra una reserva para un reloj activo, congelando
// nombre, modelo y precio. Si se crea ya confirmada, retiene una unidad.
func createReservation(ctx context.Context, in ReservationInput, actor string, now time.Time) (*Reservation, error) {
	if msg := validateReservationInput(&in, now); msg != "" {
		return nil, errBadRequest(msg)
	}
	watchID, _ := primitive.ObjectIDFromHex(in.WatchID)
	watch, err := findWatch(ctx, bson.M{"_id": watchID, "archivedAt": nil})
	if err != nil {
		return nil, err
	}

	price := watch.PriceARS
	if in.PriceAtReservation > 0 {
		price = in.PriceAtReservation
	}
	if price > 0 && in.DepositAmount > price {
		return nil, errBadRequest("La seña no puede superar el precio")
	}

	res := Reservation{
		WatchID:            watch.ID,
		WatchNameSnapshot:  watch.Name,
		WatchModelSnapshot: watch.Model,
		PriceAtReservation: price,
		CustomerName:       in.CustomerName,
		Phone:              in.Phone,
		Email:              in.Email,
		Status:             in.Status,
		DepositAmount:      in.DepositAmount,
		Notes:              in.Notes,
		ExpiresAt:          in.ExpiresAt,
		CreatedAt:          now,
		UpdatedAt:          now,
		Source:             ReservationSourceAdmin,
		WatchImageSnapshot: watch.Image,
	}
	if in.InquiryID != "" {
		iid, _ := primitive.ObjectIDFromHex(in.InquiryID)
		res.InquiryID = &iid
	}

	var heldWatch *Watch
	if holdsStock(res.Status) {
		if heldWatch, err = holdUnit(ctx, watch.ID); err != nil {
			return nil, err
		}
		res.StockHeld = true
		res.ConfirmedAt = &now
	}
	ins, err := reservationsCol().InsertOne(ctx, res)
	if err != nil {
		if res.StockHeld {
			releaseUnit(ctx, watch.ID) // compensación (stock neto sin cambios: no se anota)
		}
		return nil, err
	}
	res.ID = ins.InsertedID.(primitive.ObjectID)
	if heldWatch != nil {
		recordMovement(ctx, movementFromWatch(heldWatch, MoveReservationHold, -1, &res.ID, "Reserva creada confirmada", actor))
	}
	return &res, nil
}

// transitionOpts ajusta un cambio de estado.
type transitionOpts struct {
	DepositAmount *int64 // nueva seña (opcional)
	DepositMethod string // obligatorio al pasar a deposit_paid
	// AllowedFrom restringe desde qué estados se acepta el cambio (además de
	// las transiciones válidas). Se usa para la cancelación por el cliente,
	// que solo puede cancelar reservas pendientes.
	AllowedFrom []string
	// CustomerID, si viene, exige que la reserva sea de ese cliente. Una
	// reserva ajena responde 404 (no se revela que existe).
	CustomerID *primitive.ObjectID
	// Actor queda registrado en el movimiento de stock (email del admin).
	Actor string
}

// transitionReservation cambia el estado de una reserva aplicando el efecto
// sobre el stock que corresponde.
func transitionReservation(ctx context.Context, id primitive.ObjectID, to string, opts transitionOpts, now time.Time) (*Reservation, error) {
	if !validReservationStatus(to) {
		return nil, errBadRequest("Estado de reserva inválido")
	}
	// Si ya venció, se marca como vencida antes de cualquier otro cambio.
	if _, err := expireDueReservations(ctx, now, &id); err != nil {
		return nil, err
	}
	r, err := findReservation(ctx, id)
	if err != nil {
		return nil, err
	}
	if opts.CustomerID != nil && (r.CustomerID == nil || *r.CustomerID != *opts.CustomerID) {
		return nil, errNotFound("Reserva no encontrada")
	}
	if opts.AllowedFrom != nil && !containsString(opts.AllowedFrom, r.Status) {
		return nil, errConflict("Esta reserva ya no se puede cancelar desde la web. Contactanos para cancelarla.")
	}
	if !canTransition(r.Status, to) {
		return nil, errCode(http.StatusConflict, CodeInvalidTransition, "No se puede pasar una reserva de \""+r.Status+"\" a \""+to+"\"")
	}

	deposit := r.DepositAmount
	if opts.DepositAmount != nil {
		deposit = *opts.DepositAmount
	}
	if deposit < 0 || deposit > maxPriceARS || (r.PriceAtReservation > 0 && deposit > r.PriceAtReservation) {
		return nil, errBadRequest("La seña es inválida o supera el precio")
	}
	set := bson.M{"status": to, "updatedAt": now}
	if to == ReservationDepositPaid {
		if deposit <= 0 {
			return nil, errBadRequest("Indicá el monto de la seña recibida")
		}
		if !validDepositMethod(opts.DepositMethod) {
			return nil, errBadRequest("Indicá cómo se recibió la seña (efectivo, transferencia, tarjeta en el local u otro)")
		}
		set["depositMethod"] = opts.DepositMethod
		set["depositPaidAt"] = now
	}
	set["depositAmount"] = deposit
	// Fecha de cada estado (depositPaidAt ya se fijó arriba).
	switch to {
	case ReservationConfirmed:
		set["confirmedAt"] = now
	case ReservationCompleted:
		set["completedAt"] = now
	case ReservationCancelled:
		set["cancelledAt"] = now
	case ReservationExpired:
		set["expiredAt"] = now
	}

	needsHold := holdsStock(to) && !r.StockHeld
	needsRelease := (to == ReservationCancelled || to == ReservationExpired) && r.StockHeld
	// completed: la unidad retenida pasa a estar vendida (no vuelve al stock).
	stockHeldAfter := holdsStock(to)

	var heldWatch *Watch
	if needsHold {
		if heldWatch, err = holdUnit(ctx, r.WatchID); err != nil {
			return nil, err
		}
	}
	set["stockHeld"] = stockHeldAfter
	var out Reservation
	err = reservationsCol().FindOneAndUpdate(ctx,
		bson.M{"_id": r.ID, "status": r.Status, "stockHeld": r.StockHeld},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&out)
	if err != nil {
		if needsHold {
			releaseUnit(ctx, r.WatchID) // compensación (stock neto sin cambios: no se anota)
		}
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errCode(http.StatusConflict, CodeConcurrentUpdate, "La reserva fue modificada al mismo tiempo. Recargá y volvé a intentar.")
		}
		return nil, err
	}
	actor := opts.Actor
	if actor == "" {
		actor = systemActor
	}
	switch {
	case heldWatch != nil:
		recordMovement(ctx, movementFromWatch(heldWatch, MoveReservationHold, -1, &r.ID, "Reserva confirmada", actor))
	case needsRelease:
		reason := "Reserva cancelada"
		if to == ReservationExpired {
			reason = "Reserva vencida"
		}
		releaseAndRecord(ctx, r, reason, actor)
	case to == ReservationCompleted && r.StockHeld:
		// La unidad ya se había descontado al confirmar: la venta se anota
		// con delta 0 para trazabilidad, sin descontar dos veces.
		if w, err := findWatch(ctx, bson.M{"_id": r.WatchID}); err == nil {
			recordMovement(ctx, movementFromWatch(w, MoveSale, 0, &r.ID, "Venta completada en el local", actor))
		}
	}
	return &out, nil
}

// expireDueReservations marca como vencidas las reservas pendientes o
// confirmadas cuyo expiresAt ya pasó, y devuelve al stock las unidades que
// tenían retenidas. Si only != nil, procesa solo esa reserva.
//
// Cada reserva se vence con un FindOneAndUpdate condicional (atómico): si una
// cancelación manual ocurre al mismo tiempo, solo una de las dos operaciones
// encuentra la reserva en estado activo, y la unidad se devuelve una sola vez.
func expireDueReservations(ctx context.Context, now time.Time, only *primitive.ObjectID) (int, error) {
	filter := bson.M{
		"status":    bson.M{"$in": bson.A{ReservationPending, ReservationConfirmed}},
		"expiresAt": bson.M{"$ne": nil, "$lte": now},
	}
	if only != nil {
		filter["_id"] = *only
	}
	n := 0
	for {
		var before Reservation
		err := reservationsCol().FindOneAndUpdate(ctx, filter,
			bson.M{"$set": bson.M{"status": ReservationExpired, "stockHeld": false, "updatedAt": now, "expiredAt": now}},
			options.FindOneAndUpdate().SetReturnDocument(options.Before),
		).Decode(&before)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		if before.StockHeld {
			releaseAndRecord(ctx, &before, "Reserva vencida automáticamente", systemActor)
		}
		n++
	}
}

// startReservationSweeper vence reservas periódicamente en segundo plano.
// Además, el listado de reservas del Admin y cada cambio de estado ejecutan
// el vencimiento en el momento, así que el intervalo solo define cuánto
// puede tardar en volver al stock una unidad si nadie usa el Admin.
func startReservationSweeper(ctx context.Context, every time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c, cancel := context.WithTimeout(ctx, 30*time.Second)
				if n, err := expireDueReservations(c, time.Now(), nil); err != nil {
					log.Println("No se pudieron vencer reservas:", err)
				} else if n > 0 {
					log.Printf("Reservas vencidas automáticamente: %d", n)
				}
				cancel()
			}
		}
	}()
}

// ---------- Reservas solicitadas por clientes ----------

const (
	ReservationSourceWeb   = "web"
	ReservationSourceAdmin = "admin"

	// Una solicitud web pendiente vence sola si el local no la confirma.
	webPendingDays = 7
	// Límite de solicitudes pendientes simultáneas por cliente (anti-abuso).
	maxPendingPerCustomer = 3
)

// openStatuses: reservas que siguen activas para el cliente.
var openStatuses = bson.A{ReservationPending, ReservationConfirmed, ReservationDepositPaid}

// CustomerReservationRequest es el cuerpo de POST /api/reservations.
type CustomerReservationRequest struct {
	WatchID       string `json:"watchId"`
	TermsAccepted bool   `json:"termsAccepted"`
	Notes         string `json:"notes"`
}

// createCustomerReservation registra una SOLICITUD de reserva: queda
// pendiente y NO descuenta stock. Solo el Admin puede confirmarla (y recién
// ahí se retiene la unidad, con la operación atómica de siempre).
func createCustomerReservation(ctx context.Context, cust *Customer, in CustomerReservationRequest, now time.Time) (*Reservation, error) {
	in.Notes = strings.TrimSpace(in.Notes)
	if !primitive.IsValidObjectID(strings.TrimSpace(in.WatchID)) {
		return nil, errBadRequest("Reloj inválido")
	}
	if !in.TermsAccepted {
		return nil, errBadRequest("Para solicitar la reserva tenés que aceptar los términos de reserva")
	}
	if len(in.Notes) > 500 {
		return nil, errBadRequest("El comentario es demasiado largo (máximo 500 caracteres)")
	}
	watchID, _ := primitive.ObjectIDFromHex(strings.TrimSpace(in.WatchID))
	watch, err := findWatch(ctx, bson.M{"_id": watchID, "archivedAt": nil})
	if err != nil {
		return nil, err
	}
	if watch.StockQuantity < 1 {
		return nil, errConflict("Este reloj no tiene stock en este momento. Podés consultarnos por reposición.")
	}

	dup, err := reservationsCol().CountDocuments(ctx, bson.M{"customerId": cust.ID, "watchId": watch.ID, "status": bson.M{"$in": openStatuses}})
	if err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, errConflict("Ya tenés una reserva activa para este reloj. Podés verla en Mis reservas.")
	}
	pending, err := reservationsCol().CountDocuments(ctx, bson.M{"customerId": cust.ID, "status": ReservationPending})
	if err != nil {
		return nil, err
	}
	if pending >= maxPendingPerCustomer {
		return nil, errConflict("Tenés varias solicitudes pendientes. Esperá a que las revisemos o cancelá alguna.")
	}

	expires := now.AddDate(0, 0, webPendingDays)
	res := Reservation{
		WatchID:            watch.ID,
		WatchNameSnapshot:  watch.Name,
		WatchModelSnapshot: watch.Model,
		WatchImageSnapshot: watch.Image,
		PriceAtReservation: watch.PriceARS,
		CustomerID:         &cust.ID,
		CustomerName:       cust.FirstName + " " + cust.LastName,
		Phone:              cust.Phone,
		Email:              cust.Email,
		CustomerNotes:      in.Notes,
		Status:             ReservationPending,
		ExpiresAt:          &expires,
		Source:             ReservationSourceWeb,
		TermsAcceptedAt:    &now,
		TermsVersion:       TermsVersion,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	ins, err := reservationsCol().InsertOne(ctx, res)
	if err != nil {
		return nil, err
	}
	res.ID = ins.InsertedID.(primitive.ObjectID)
	return &res, nil
}

// CustomerReservationView es lo que ve el cliente de su reserva: sin notas
// internas del local, sin datos de stock interno ni de otros clientes.
type CustomerReservationView struct {
	ID                 string     `json:"id"`
	WatchID            string     `json:"watchId"`
	WatchName          string     `json:"watchName"`
	WatchModel         string     `json:"watchModel"`
	WatchImage         string     `json:"watchImage"`
	PriceAtReservation int64      `json:"priceAtReservation"`
	Status             string     `json:"status"`
	DepositAmount      int64      `json:"depositAmount"`
	DepositPaidAt      *time.Time `json:"depositPaidAt,omitempty"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	CustomerNotes      string     `json:"customerNotes,omitempty"`
	CanCancel          bool       `json:"canCancel"`
	CreatedAt          time.Time  `json:"createdAt"`
}

func customerView(r Reservation) CustomerReservationView {
	v := CustomerReservationView{
		ID: r.ID.Hex(), WatchID: r.WatchID.Hex(), WatchName: r.WatchNameSnapshot,
		WatchModel: r.WatchModelSnapshot, WatchImage: r.WatchImageSnapshot,
		PriceAtReservation: r.PriceAtReservation, Status: r.Status,
		DepositAmount: r.DepositAmount, DepositPaidAt: r.DepositPaidAt, CustomerNotes: r.CustomerNotes,
		CanCancel: r.Status == ReservationPending, CreatedAt: r.CreatedAt,
	}
	if expirable(r.Status) {
		v.ExpiresAt = r.ExpiresAt
	}
	return v
}

// listCustomerReservations devuelve SOLO las reservas del cliente indicado.
func listCustomerReservations(ctx context.Context, customerID primitive.ObjectID, now time.Time) ([]CustomerReservationView, error) {
	if _, err := expireDueReservations(ctx, now, nil); err != nil {
		return nil, err
	}
	cur, err := reservationsCol().Find(ctx, bson.M{"customerId": customerID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var items []Reservation
	if err := cur.All(ctx, &items); err != nil {
		return nil, err
	}
	out := make([]CustomerReservationView, 0, len(items))
	for _, r := range items {
		out = append(out, customerView(r))
	}
	return out, nil
}
