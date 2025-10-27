#!/usr/bin/env bats

@test "can run asconfig" {
  asconfig --help
  [ "$?" -eq 0 ]
}