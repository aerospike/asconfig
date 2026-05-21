#!/usr/bin/env bats

@test "can run asconfig" {
  asconfig --help
  [ "$?" -eq 0 ]
}

@test "asconfig reports version" {
  asconfig --version
  [ "$?" -eq 0 ]
}
