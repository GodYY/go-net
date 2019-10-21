package session

// Listener is a generic listener for stream-oriented socket-session.
type Listener interface {
	// Waits for the next connection, constructs and returns the socket-session.
	Accept() (Session, error)

	// Close the listener, any blocked Accept() operation will be unblocked and return error.
	Close() error

	// Return name of the network. ("tcp", "tcp4")
	Network() string

	// Return string form of the address listening.
	Addr() string
}

// Codecs is a generic coder-decoder for all type of socket-session.
type Codecs interface {
	// Code the message giving.
	Encode(o interface{}) (Message, error)

	// Decode try to decode the byte slice giving to a message object.
	Decode(bytes []byte) (interface{}, error)
}
