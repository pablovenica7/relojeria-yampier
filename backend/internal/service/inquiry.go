package service

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
)

type inquiryStore interface {
	Insert(ctx context.Context, q *domain.Inquiry) error
	List(ctx context.Context, status string, watchID *primitive.ObjectID, limit int) ([]domain.Inquiry, error)
	Update(ctx context.Context, id primitive.ObjectID, c domain.InquiryChange) (*domain.Inquiry, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
}

// InquiryService gestiona consultas. Una consulta solo expresa interés:
// nunca modifica el stock.
type InquiryService struct {
	inquiries inquiryStore
	watches   watchFinder
	now       Clock
}

func NewInquiryService(inquiries inquiryStore, watches watchFinder, now Clock) *InquiryService {
	return &InquiryService{inquiries: inquiries, watches: watches, now: clockOrNow(now)}
}

func validateInquiry(in *dto.InquiryRequest) string {
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
	case in.Email == "" || !domain.ValidEmail(in.Email):
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

// Create guarda una consulta. Si viene de un reloj, el nombre y el modelo se
// toman de la base (no del texto del cliente) y se guardan como snapshot.
func (s *InquiryService) Create(ctx context.Context, in dto.InquiryRequest) (*domain.Inquiry, error) {
	if msg := validateInquiry(&in); msg != "" {
		return nil, domain.Validation(msg)
	}
	now := s.now()
	q := &domain.Inquiry{Name: in.Name, Email: in.Email, Phone: in.Phone, Message: in.Message,
		Status: domain.InquiryNew, CreatedAt: now, UpdatedAt: now}
	if in.WatchID != "" {
		id, _ := primitive.ObjectIDFromHex(in.WatchID)
		w, err := s.watches.FindByID(ctx, id, true)
		if isNotFound(err) {
			return nil, errWatchNotFound
		}
		if err != nil {
			return nil, err
		}
		q.WatchID, q.WatchNameSnapshot, q.WatchModelSnapshot = &w.ID, w.Name, w.Model
	}
	if err := s.inquiries.Insert(ctx, q); err != nil {
		return nil, err
	}
	return q, nil
}

// List devuelve consultas (más recientes primero).
func (s *InquiryService) List(ctx context.Context, status string, watchID *primitive.ObjectID) ([]domain.Inquiry, error) {
	if status != "" && !domain.ValidInquiryStatus(status) {
		return nil, domain.Validation("Estado inválido")
	}
	return s.inquiries.List(ctx, status, watchID, 500)
}

// Update cambia estado y/o notas internas. contactedAt/closedAt se fijan la
// primera vez que la consulta llega a ese estado.
func (s *InquiryService) Update(ctx context.Context, id primitive.ObjectID, in dto.InquiryUpdateRequest) (*domain.Inquiry, error) {
	change := domain.InquiryChange{At: s.now()}
	if in.Status != nil {
		if !domain.ValidInquiryStatus(*in.Status) {
			return nil, domain.Validation("Estado inválido")
		}
		change.Status = in.Status
	}
	if in.AdminNotes != nil {
		notes := strings.TrimSpace(*in.AdminNotes)
		if len(notes) > 2000 {
			return nil, domain.Validation("Las notas son demasiado largas (máximo 2000 caracteres)")
		}
		change.AdminNotes = &notes
	}
	if change.Status == nil && change.AdminNotes == nil {
		return nil, domain.Validation("No hay cambios para guardar")
	}
	q, err := s.inquiries.Update(ctx, id, change)
	if isNotFound(err) {
		return nil, domain.NotFound("", "Consulta no encontrada")
	}
	return q, err
}

// Delete borra una consulta (spam o errores).
func (s *InquiryService) Delete(ctx context.Context, id primitive.ObjectID) error {
	if err := s.inquiries.Delete(ctx, id); err != nil {
		if isNotFound(err) {
			return domain.NotFound("", "Consulta no encontrada")
		}
		return err
	}
	return nil
}
