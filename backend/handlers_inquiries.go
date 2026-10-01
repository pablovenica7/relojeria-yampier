package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Validación razonable de email: no pretende cubrir el RFC completo (nada lo
// hace de forma práctica), solo rechazar valores claramente inválidos.
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func validateInquiry(in *InquiryInput) string {
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Message = strings.TrimSpace(in.Message)
	in.WatchID = strings.TrimSpace(in.WatchID)

	switch {
	case in.Name == "":
		return "Ingresá tu nombre"
	case len(in.Name) > 120:
		return "El nombre es demasiado largo"
	case in.Email == "" || !emailRe.MatchString(in.Email):
		return "Ingresá un email válido"
	case len(in.Email) > 190:
		return "El email es demasiado largo"
	case len(in.Phone) > 40:
		return "El teléfono es demasiado largo"
	case len(in.Message) < 10:
		return "Contanos un poco más en el mensaje (mínimo 10 caracteres)"
	case len(in.Message) > 2000:
		return "El mensaje es demasiado largo (máximo 2000 caracteres)"
	case in.WatchID != "" && !primitive.IsValidObjectID(in.WatchID):
		return "Reloj inválido"
	}
	return ""
}

// buildInquiry arma la consulta a guardar. Si viene de un reloj concreto,
// el nombre se toma de la base (no del texto enviado por el cliente) y se
// guarda como snapshot: la consulta sigue identificando el reloj aunque
// después se renombre.
func buildInquiry(ctx context.Context, in InquiryInput, now time.Time) (Inquiry, error) {
	q := Inquiry{
		Name: in.Name, Email: in.Email, Phone: in.Phone, Message: in.Message,
		Status: InquiryNew, CreatedAt: now, UpdatedAt: now,
	}
	if in.WatchID != "" {
		id, _ := primitive.ObjectIDFromHex(in.WatchID)
		watch, err := findWatch(ctx, bson.M{"_id": id, "archivedAt": nil})
		if err != nil {
			return q, err
		}
		q.WatchID = &watch.ID
		q.WatchNameSnapshot = watch.Name
		q.WatchModelSnapshot = watch.Model
	}
	return q, nil
}

// POST /api/inquiries  (público) — formulario de Contacto o consulta por un reloj.
// La validación del frontend es solo para UX: acá se vuelve a validar todo.
// Una consulta NUNCA modifica el stock.
func postInquiry(w http.ResponseWriter, r *http.Request) {
	var in InquiryInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	if msg := validateInquiry(&in); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	q, err := buildInquiry(ctx, in, time.Now())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if _, err := inquiriesCol().InsertOne(ctx, q); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// GET /api/admin/inquiries?status=new|contacted|closed&watchId=  (protegido)
func listInquiries(w http.ResponseWriter, r *http.Request) {
	filter := bson.M{}
	if s := r.URL.Query().Get("status"); s != "" {
		if !validInquiryStatus(s) {
			writeError(w, http.StatusBadRequest, "Estado inválido")
			return
		}
		filter["status"] = s
	}
	if s := r.URL.Query().Get("watchId"); s != "" {
		id, err := primitive.ObjectIDFromHex(s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Reloj inválido")
			return
		}
		filter["watchId"] = id
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(500)
	cur, err := inquiriesCol().Find(ctx, filter, opts)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer cur.Close(ctx)
	out := []Inquiry{}
	if err := cur.All(ctx, &out); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// PATCH /api/admin/inquiries/{id}  (protegido) — body {"status"?, "adminNotes"?}
func patchInquiry(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Consulta no encontrada")
		return
	}
	var in struct {
		Status     *string `json:"status"`
		AdminNotes *string `json:"adminNotes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}
	set := bson.M{"updatedAt": time.Now()}
	if in.Status != nil {
		if !validInquiryStatus(*in.Status) {
			writeError(w, http.StatusBadRequest, "Estado inválido")
			return
		}
		set["status"] = *in.Status
	}
	if in.AdminNotes != nil {
		notes := strings.TrimSpace(*in.AdminNotes)
		if len(notes) > 2000 {
			writeError(w, http.StatusBadRequest, "Las notas son demasiado largas (máximo 2000 caracteres)")
			return
		}
		set["adminNotes"] = notes
	}
	if len(set) == 1 {
		writeError(w, http.StatusBadRequest, "No hay cambios para guardar")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	// contactedAt / closedAt: se fijan la primera vez que la consulta llega a
	// ese estado (pipeline con $ifNull: no se pisan si ya existían).
	// En un pipeline, un string que empieza con "$" se interpretaría como
	// referencia a un campo (ej: una nota "$100 de seña"): todo valor del
	// usuario va envuelto en $literal.
	literal := bson.M{}
	for k, v := range set {
		literal[k] = bson.M{"$literal": v}
	}
	update := bson.A{bson.M{"$set": literal}}
	if in.Status != nil && *in.Status == InquiryContacted {
		update = append(update, bson.M{"$set": bson.M{"contactedAt": bson.M{"$ifNull": bson.A{"$contactedAt", set["updatedAt"]}}}})
	}
	if in.Status != nil && *in.Status == InquiryClosed {
		update = append(update, bson.M{"$set": bson.M{"closedAt": bson.M{"$ifNull": bson.A{"$closedAt", set["updatedAt"]}}}})
	}
	var out Inquiry
	err := inquiriesCol().FindOneAndUpdate(ctx, bson.M{"_id": id}, update,
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&out)
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeError(w, http.StatusNotFound, "Consulta no encontrada")
		return
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /api/admin/inquiries/{id}  (protegido) — para spam o datos cargados por error.
func deleteInquiry(w http.ResponseWriter, r *http.Request) {
	id, ok := parseObjectID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "Consulta no encontrada")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	res, err := inquiriesCol().DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if res.DeletedCount == 0 {
		writeError(w, http.StatusNotFound, "Consulta no encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
