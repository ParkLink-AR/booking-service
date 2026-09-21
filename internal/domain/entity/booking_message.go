package entity

import "time"

// BookingMessage is one chat message exchanged between a booking's driver and its
// spot's owner -- e.g. access instructions before the driver arrives. Either side can
// send one once the booking exists; see usecase.SendMessageUseCase for who's allowed.
type BookingMessage struct {
	ID        string
	BookingID string
	SenderID  string
	Body      string
	CreatedAt time.Time
}
