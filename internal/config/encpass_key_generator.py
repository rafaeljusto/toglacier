#!/usr/bin/env python
# -*- coding: utf-8 -*-

"""This module was created to generate a Go file with random key parameters."""

from __future__ import print_function
from random import randint, seed

import argparse
import os

TEMPLATE = """package config

// passwordKey returns the shared secret used to encrypt and decrypt the
// passwords. For safety, you should change this numbers before deploying this
// project. It's important that the secret has 16 characters, so that we can
// generate an AES-128.
func passwordKey() []byte {
	a0 := []byte{0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X}
	a1 := []byte{0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X}
	a2 := []byte{0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X}
	a3 := []byte{0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X, 0x%02X}

	result := make([]byte, 16)
	for i := 0; i < len(result); i++ {
		result[i] = (((a0[i] & a1[i]) ^ a2[i]) | a3[i])
	}

	return result
}
"""

def generate(write):
    """Generate Go file with random key parameters."""

    seed()
    numbers = []

    for _ in range(64):
        numbers.append(randint(0, 255))

    if write:
        # if you fork the project you will probably need to change this path
        path = os.environ["GOPATH"].split(":")[0] + \
            "/src/github.com/rafaeljusto/toglacier/internal/config/encpass_key.go"

        output = open(path, "w")
        output.write(TEMPLATE % tuple(numbers))
        output.close()

    else:
        print(TEMPLATE % tuple(numbers))

def main():
    """Main program"""

    parser = argparse.ArgumentParser(description="Generate Go file with random key parameters.")
    parser.add_argument("-w", "--write", \
        action="store_true", \
        default=False, \
        help="Write file instead of std output")
    args = parser.parse_args()
    generate(args.write)

if __name__ == "__main__":
    main()
