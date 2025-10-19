package types

type TuiConf struct {
	RefreshMs       int     `koanf:"refresh_ms"`
	RsWarnPct       float64 `koanf:"rs_threshold_warn_pct"`
	RsCritPct       float64 `koanf:"rs_threshold_crit_pct"`
	VitWarnPct      float64 `koanf:"vit_threshold_warn_pct"`
	VitCritPct      float64 `koanf:"vit_threshold_crit_pct"`
	EnableLogOutput bool    `koanf:"enable_log_output"`
}

type RadioConf struct {
	Address     string  `koanf:"address"`
	DeviceIndex int     `koanf:"device_index"`
	Gain        int     `koanf:"gain"`
	Frequency   float64 `koanf:"frequency"`
	SampleRate  float64 `koanf:"sample_rate"`
	SampleType  string  `koanf:"sample_type"`
	Decimation  string  `koanf:"decimation"`
	Name        string  `koanf:"name"`
}
