package phonewave

// DeliveryLogger records D-Mail delivery events.
type DeliveryLogger interface {
	Delivered(kind, from, to string)
	Removed(from string)
	Failed(kind, from, reason string)
	Retried(kind, from, to string)
}
