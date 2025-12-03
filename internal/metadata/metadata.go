package metadata

type Annotations struct {
	Custom CustomMetadata
}

type CustomMetadata struct {
	Libarary  bool
	ID        string
	AVDID     string `mapstructure:"avd_id"`
	Provider  string
	Service   string
	ShortCode string `mapstructure:"short_code"`
	Aliases   []string
}
