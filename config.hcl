
agc {
  gain      = 1
  max_gain  = 4000
  rate      = 0.01
  reference = 0.5
}

clockrecovery {
  alpha       = 0.0037
  mu          = 0.5
  omega_limit = 0.005
}

radio {
  address     = "10.0.2.30:1234"
  decimation  = 1
  driver      = "rtltcp"
  frequency   = 1694100000
  gain        = 5
  name        = ""
  sample_rate = 2048000
}

tui {
  enable_log_output      = true
  refresh_ms             = 500
  rs_threshold_crit_pct  = 5
  rs_threshold_warn_pct  = 2
  vit_threshold_crit_pct = 5
  vit_threshold_warn_pct = 3
}

viterbi {
  max_errors = 500
}

xrit {
  chunk_size               = 66560
  decimation_factor        = 1
  do_fft                   = true
  lowpass_transition_width = 200000
  pll_alpha                = 0.001
  rrc_alpha                = 0.3
  rrc_taps                 = 31
  symbol_rate              = 927000
}

xritframe {
  frame_size      = 1024
  last_frame_size = 8
}
