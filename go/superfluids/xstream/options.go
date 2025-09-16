package xstream

// MuxOptions used to control the behaviour of the mux
type MuxOptions struct {
	//caughtupFunc   func()
	//subscribedFunc func()
	blockUntilLive bool
}
type MuxOption func(*MuxOptions)

func MuxBlockUntilLive() MuxOption {
	return func(o *MuxOptions) {
		o.blockUntilLive = true
	}
}
