package kafkaexporter

import (
	"strings"

	"github.com/Shopify/sarama"
)

// Compression defines the compression method and the compression level.
type Compression struct {
	Codec string `mapstructure:"codec"`
	Level int    `mapstructure:"level"`
}

func configureCompression(comp Compression, saramaConfig *sarama.Config) {
	switch strings.ToLower(comp.Codec) {
	case "none":
		saramaConfig.Producer.Compression = sarama.CompressionNone
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	}
	saramaConfig.Producer.CompressionLevel = comp.Level
}
