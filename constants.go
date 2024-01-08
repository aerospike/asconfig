package main

type Format string

const (
	Invalid  Format = ""
	YAML     Format = "yaml"
	AsConfig Format = "asconfig"
)
