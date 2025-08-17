#! /bin/bash

sudo apt install mockgen

mockgen -source=./repositories/configfile_repository.go -destination=./repositories/mock_repositories/configfile_repository_mock.go -package=mock_repositories
