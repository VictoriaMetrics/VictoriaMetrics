package errorlint

type Option func()

func WithAllowedErrors(ap []AllowPair) Option {
	return func() {
		allowedMapAppend(ap)
	}
}

func WithAllowedWildcard(ap []AllowPair) Option {
	return func() {
		allowedWildcardAppend(ap)
	}
}
