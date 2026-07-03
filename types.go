package unisms

// SendRequest describes an SMS to be sent via Client.Send.
type SendRequest struct {
	// Recipient is the destination mobile number. Required.
	Recipient string

	// Content is the SMS message body. Required.
	Content string

	// SenderID is the sender name shown to the recipient. Optional; the
	// API defaults this to "UniSMS" when left blank.
	SenderID string

	// Metadata is an arbitrary set of key/value pairs attached to the
	// message for tracking purposes, echoed back by the API. Optional;
	// omitted entirely from the request body when nil.
	Metadata map[string]interface{}
}

// sendPayload is the JSON wire shape posted to the UniSMS API. It is kept
// separate from SendRequest so metadata can be omitted entirely (via
// omitempty) rather than sent as null when not provided.
type sendPayload struct {
	Recipient string                 `json:"recipient"`
	Content   string                 `json:"content"`
	SenderID  string                 `json:"sender_id"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Response is the decoded JSON body returned by the UniSMS API for both
// the send and get-message-status operations. The API does not guarantee
// a fixed schema, so responses are exposed as a plain map rather than a
// rigid struct.
type Response map[string]interface{}

// String returns the string value stored at key, or "" if the key is
// absent or is not a string. It is a convenience accessor for common
// fields such as response["message"]["reference_id"] style lookups at
// the top level.
func (r Response) String(key string) string {
	v, ok := r[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// Get returns the raw value stored at key and whether it was present.
func (r Response) Get(key string) (interface{}, bool) {
	v, ok := r[key]
	return v, ok
}
