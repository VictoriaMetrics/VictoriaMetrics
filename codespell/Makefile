# These commands must be run from the VictoriaMetrics repository root.

# Builds spellcheck image.
codespell:
	@docker build codespell -t codespell

# Runs cspell container commands.
codespell-check: codespell
	@-docker run \
		--mount type=bind,src="$(PWD)",dst=/vm \
		--rm \
		codespell \
			--ignore-words=/vm/codespell/stopwords \
			--skip='*/node_modules/*,*/vmdocs/*,*/vendor/*,*.js,*.pb.go,*.qtpl.go' /vm
