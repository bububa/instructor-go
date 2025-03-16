package instructor

const (
	DefaultMaxRetries = 3
	DefaultValidator  = false
	DefaultVerbose    = false
)

type Option func(o *Options)

type Options struct {
	provider   Provider
	mode       Mode
	enc        Encoder
	streamEnc  StreamEncoder
	maxRetries int
	validate   bool
	verbose    bool
	// Provider specific options:
}

var defaultOptions = Options{
	mode:       ModeDefault,
	maxRetries: DefaultMaxRetries,
	validate:   DefaultValidator,
	verbose:    DefaultVerbose,
}

func WithProvider(provider Provider) Option {
	return func(o *Options) {
		o.provider = provider
	}
}

func WithMode(mode Mode) Option {
	return func(o *Options) {
		o.mode = mode
	}
}

func WithEncoder(enc Encoder) Option {
	return func(o *Options) {
		o.enc = enc
	}
}

func WithStreamEncoder(enc StreamEncoder) Option {
	return func(o *Options) {
		o.streamEnc = enc
	}
}

func WithMaxRetries(maxRetries int) Option {
	return func(o *Options) {
		o.maxRetries = maxRetries
	}
}

func WithValidation() Option {
	return func(o *Options) {
		o.validate = true
	}
}

func WithVerbose() Option {
	return func(o *Options) {
		o.verbose = true
	}
}

func (i Options) Provider() Provider {
	return i.provider
}

func (i Options) Mode() Mode {
	return i.mode
}

func (i *Options) SetEncoder(enc Encoder) {
	i.enc = enc
}

func (i *Options) SetStreamEncoder(enc StreamEncoder) {
	i.streamEnc = enc
}

func (i Options) Encoder() Encoder {
	return i.enc
}

func (i Options) StreamEncoder() StreamEncoder {
	return i.streamEnc
}

func (i Options) MaxRetries() int {
	return i.maxRetries
}

func (i Options) Validate() bool {
	return i.validate
}

func (i Options) Verbose() bool {
	return i.verbose
}
