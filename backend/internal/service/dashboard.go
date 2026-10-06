package service

import (
	"context"
	"time"

	"relojeria-yampier/internal/domain"
	"relojeria-yampier/internal/dto"
)

type dashboardReservations interface {
	CountByStatus(ctx context.Context, status string) (int64, error)
	ListExpiringBetween(ctx context.Context, from, to time.Time, limit int) ([]domain.Reservation, error)
}

type dashboardWatches interface {
	Count(ctx context.Context, scope, availability string) (int64, error)
}

type dashboardInquiries interface {
	CountByStatus(ctx context.Context, status string) (int64, error)
}

// DashboardService arma los números de la operación diaria del Admin.
type DashboardService struct {
	reservations dashboardReservations
	watches      dashboardWatches
	inquiries    dashboardInquiries
	expirer      *ReservationService
	now          Clock
}

func NewDashboardService(reservations dashboardReservations, watches dashboardWatches, inquiries dashboardInquiries, expirer *ReservationService, now Clock) *DashboardService {
	return &DashboardService{reservations: reservations, watches: watches, inquiries: inquiries, expirer: expirer, now: clockOrNow(now)}
}

// Summary vence lo que corresponda y devuelve las métricas.
func (s *DashboardService) Summary(ctx context.Context) (*dto.Dashboard, error) {
	if _, err := s.expirer.ExpireDue(ctx, nil); err != nil {
		return nil, err
	}
	d := &dto.Dashboard{}
	var err error
	for _, c := range []struct {
		dst    *int64
		status string
	}{
		{&d.PendingReservations, domain.ReservationPending},
		{&d.ConfirmedReservations, domain.ReservationConfirmed},
		{&d.DepositPaidReservations, domain.ReservationDepositPaid},
	} {
		if *c.dst, err = s.reservations.CountByStatus(ctx, c.status); err != nil {
			return nil, err
		}
	}
	if d.OutOfStock, err = s.watches.Count(ctx, domain.ScopeActive, domain.AvailabilityOutOfStock); err != nil {
		return nil, err
	}
	if d.LastUnit, err = s.watches.Count(ctx, domain.ScopeActive, domain.AvailabilityLastUnit); err != nil {
		return nil, err
	}
	if d.NewInquiries, err = s.inquiries.CountByStatus(ctx, domain.InquiryNew); err != nil {
		return nil, err
	}
	now := s.now()
	items, err := s.reservations.ListExpiringBetween(ctx, now, now.Add(48*time.Hour), 10)
	if err != nil {
		return nil, err
	}
	d.ExpiringSoon = make([]dto.ExpiringReservation, 0, len(items))
	for _, it := range items {
		d.ExpiringSoon = append(d.ExpiringSoon, dto.ExpiringReservation{
			ID: it.ID.Hex(), WatchName: it.WatchNameSnapshot, CustomerName: it.CustomerName,
			Status: it.Status, ExpiresAt: *it.ExpiresAt,
		})
	}
	return d, nil
}
