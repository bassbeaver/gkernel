package event

type ApplicationLaunched struct {
	Propagator
	containerAccessor
}

//--------------------

func NewApplicationLaunched(containerAccessorObj containerAccessor) *ApplicationLaunched {
	return &ApplicationLaunched{
		containerAccessor: containerAccessorObj,
	}
}
