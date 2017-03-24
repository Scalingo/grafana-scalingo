#!/bin/bash

git clone git@github.com:torkelo/private.git ~/private-repo

gpg --allow-secret-key-import --import ~/private-repo/signing/private.key

cp ./scripts/build/rpmmacros ~/.rpmmacros

./scripts/build/sign_expect $GPG_KEY_PASSWORD dist/*.rpm
