package config

type ConfigFile struct {
	Agc           Agc           `json:"agc" hcl:"agc,block"`
	Clockrecovery Clockrecovery `json:"clockrecovery" hcl:"clockrecovery,block"`
	Radio         Radio         `json:"radio" hcl:"radio,block"`
	Tui           Tui           `json:"tui" hcl:"tui,block"`
	Viterbi       Viterbi       `json:"viterbi" hcl:"viterbi,block"`
	Xrit          Xrit          `json:"xrit" hcl:"xrit,block"`
	Xritframe     Xritframe     `json:"xritframe" hcl:"xritframe,block"`
}

type Agc struct {
	Gain      int     `json:"gain" hcl:"gain"`
	MaxGain   int     `json:"max_gain" hcl:"max_gain"`
	Rate      float64 `json:"rate" hcl:"rate"`
	Reference float64 `json:"reference" hcl:"reference"`
}

type Clockrecovery struct {
	Alpha      float64 `json:"alpha" hcl:"alpha"`
	Mu         float64 `json:"mu" hcl:"mu"`
	OmegaLimit float64 `json:"omega_limit" hcl:"omega_limit"`
}

type Radio struct {
	Address    string `json:"address" hcl:"address"`
	Decimation int    `json:"decimation" hcl:"decimation"`
	Driver     string `json:"driver" hcl:"driver"`
	Frequency  int    `json:"frequency" hcl:"frequency"`
	Gain       int    `json:"gain" hcl:"gain"`
	Name       string `json:"name" hcl:"name"`
	SampleRate int    `json:"sample_rate" hcl:"sample_rate"`
}

type Tui struct {
	EnableLogOutput     bool `json:"enable_log_output" hcl:"enable_log_output"`
	RefreshMs           int  `json:"refresh_ms" hcl:"refresh_ms"`
	RsThresholdCritPct  int  `json:"rs_threshold_crit_pct" hcl:"rs_threshold_crit_pct"`
	RsThresholdWarnPct  int  `json:"rs_threshold_warn_pct" hcl:"rs_threshold_warn_pct"`
	VitThresholdCritPct int  `json:"vit_threshold_crit_pct" hcl:"vit_threshold_crit_pct"`
	VitThresholdWarnPct int  `json:"vit_threshold_warn_pct" hcl:"vit_threshold_warn_pct"`
}

type Viterbi struct {
	MaxErrors int `json:"max_errors" hcl:"max_errors"`
}

type Xrit struct {
	ChunkSize              int     `json:"chunk_size" hcl:"chunk_size"`
	DecimationFactor       int     `json:"decimation_factor" hcl:"decimation_factor"`
	DoFft                  bool    `json:"do_fft" hcl:"do_fft"`
	LowpassTransitionWidth int     `json:"lowpass_transition_width" hcl:"lowpass_transition_width"`
	PllAlpha               float64 `json:"pll_alpha" hcl:"pll_alpha"`
	RrcAlpha               float64 `json:"rrc_alpha" hcl:"rrc_alpha"`
	RrcTaps                int     `json:"rrc_taps" hcl:"rrc_taps"`
	SymbolRate             int     `json:"symbol_rate" hcl:"symbol_rate"`
}

type Xritframe struct {
	FrameSize     int `json:"frame_size" hcl:"frame_size"`
	LastFrameSize int `json:"last_frame_size" hcl:"last_frame_size"`
}
