radio  {
  driver = "rtltcp"
  address = "10.0.0.83:1234"
  device_index = 0
  gain = 5
  frequency = 1694100000
  sample_rate = 2048000
  sample_type = "complex64"
  decimation = 1
}

tui {
  refresh_ms = 500
  rs_threshold_warn_pct = 20
  rs_threshold_crit_pct = 25
  vit_threshold_warn_pct = 10
  vit_threshold_crit_pct = 15
  enable_log_output = true
}

//radio  {
//  driver = "rtlsdr"
//  device_index = 0
//  gain = 5
//  frequency = 107700000
//  sample_rate = 2400000
//  sample_type = "complex64"
//}


// Do not touch the settings below unless you know what you're doing!
agc {
  rate = 0.01
  reference = 0.5
  gain = 1.0
  max_gain = 4000
}

clock_recovery {
  mu = 0.5
  alpha = 0.0037
  omega_limit = 0.005
}

xrit {
  symbol_rate = 927000
  rrc_alpha = 0.3
  rrc_taps = 31
  lowpass_transition_width = 200000
  pll_alpha = 0.001
  decimation_factor = 1
  chunk_size = 66560
  do_fft = false
}

xrit_frame {
  frame_size = 1024
  last_frame_size = 8
}

viterbi {
  max_errors = 500
}

