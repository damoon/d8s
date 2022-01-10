disable_snapshots()
analytics_settings(enable=False)
allow_k8s_contexts(os.getenv("TILT_ALLOW_CONTEXT"))

include('./tests/Tiltfile')
