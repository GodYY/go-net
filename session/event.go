package session

type EventType int8

const (
	// message arrival.
	EventType_Message = EventType(1)
	// error occur.
	EventType_Error   = EventType(2)
	// session close.
	EventType_Close   = EventType(3)
)

const (
	close_ConnReset = "connection reset"
	close_RemoteClose = "remote session closed"
)

// Event represent events that occur during session communication.
type Event struct {
	evtType EventType
	o       interface{}
}

func (e *Event) Type() EventType { return e.evtType }

func (e *Event) Message() interface{} {
	if e.evtType == EventType_Message {
		return e.o
	}
	return nil
}

func (e *Event) Error() *Error {
	if e.evtType == EventType_Error {
		return e.o.(*Error)
	}
	return nil
}

func (e *Event) Reason() string {
	if e.evtType == EventType_Close {
		return e.o.(string)
	}
	return ""
}

func newEventMessage(msg interface{}) Event {
	if msg == nil {
		panic(ErrNilMessage)
	}

	return Event{evtType: EventType_Message, o: msg}
}

func newEventError(err *Error) Event {
	if err == nil {
		panic("nil Error")
	}
	return Event{evtType: EventType_Error, o: err}
}

func newEventClose(s string) Event {
	return Event{evtType: EventType_Close, o: s}
}


