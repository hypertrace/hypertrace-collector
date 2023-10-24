package kafkaexporter

// Compression defines the compression method and the compression level.
type Compression struct {
	Codec string `mapstructure:"codec"`
	Level int    `mapstructure:"level"`
}
