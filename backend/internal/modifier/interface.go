package modifier

type Modifier interface {
	Name() string
	New(args map[string]interface{}) (Instance, error)
}

type Instance interface{}

type UDPModifierInstance interface {
	Instance
	Process(data []byte) ([]byte, error)
}

type ErrInvalidPacket struct {
	Err error
}

func (e *ErrInvalidPacket) Error() string {
	return "invalid packet: " + e.Err.Error()
}

type ErrInvalidArgs struct {
	Err error
}

func (e *ErrInvalidArgs) Error() string {
	return "invalid args: " + e.Err.Error()
}
