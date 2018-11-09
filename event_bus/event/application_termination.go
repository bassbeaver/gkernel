package event

type ApplicationTermination struct {
	Propagator
	containerAccessor
}

//--------------------

func NewApplicationTermination(containerAccessorObj containerAccessor) *ApplicationTermination {
	return &ApplicationTermination{
		containerAccessor: containerAccessorObj,
	}
}
