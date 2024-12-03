package facts

type Fact uint8

const (
	// maximumFactValue is the value of the highest currently known Fact.
	maximumFactValue = 2

	// None is the default value for Fact.
	// Getting a Fact of type None means there are no facts for the given key.
	None Fact = 0

	// GrpcServerType is a Fact that represents a gRPC server implementation object type.
	GrpcServerType Fact = 1

	// GrpcServerStream is a Fact that represents a gRPC server stream object.
	GrpcServerStream Fact = 2
)

func (f Fact) String() string {
	switch f {
	case None:
		return "None"
	case GrpcServerType:
		return "GrpcServer"
	case GrpcServerStream:
		return "GrpcServerStream"
	default:
		return "Unknown"
	}
}
