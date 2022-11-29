# metricremover processor

The purpose of this processor is to remove metrics based on a criteria in the config.

## Config
`remove_none_metric_type`: when set to `true`, all metrics whose type is `None` will be removed. This is important since they cannot be translated by the exporters and will error out. This usually happens when old metric APIs send metrics missing the type.
