# Docker Build Targets for Klever Blockchain
# This Makefile is included from the root Makefile
# All paths are relative to the project root directory

############################
###  Docker Configuration ##
############################

DOCKER_REGISTRY := kleverapp
DOCKER_REPO := $(DOCKER_REGISTRY)/klever-go
export DOCKER_BUILDKIT := 1
BUILD_FLAGS := --no-cache --pull --build-arg arg_version=${VERSION}
DOCKER_PLATFORMS := linux/amd64,linux/arm64
DOCKER_BUILDER := klever-builder

# Conditionally add latest tag if PUSH_LATEST is set
ifdef PUSH_LATEST
LATEST_TAG_FULL := --tag $(DOCKER_REPO):${FOR_DEV}latest${FOR_TESTNET}
LATEST_TAG_VALIDATOR := --tag $(DOCKER_REPO):val-${FOR_DEV}latest${FOR_TESTNET}
LATEST_TAG_ALPINE := --tag $(DOCKER_REPO):alpine-${FOR_DEV}latest${FOR_TESTNET}
LATEST_TAG_ALPINE_VALIDATOR := --tag $(DOCKER_REPO):alpine-val-${FOR_DEV}latest${FOR_TESTNET}
else
LATEST_TAG_FULL :=
LATEST_TAG_VALIDATOR :=
LATEST_TAG_ALPINE :=
LATEST_TAG_ALPINE_VALIDATOR :=
endif

############################
###   Helper Targets     ###
############################

.PHONY: docker-vendor docker-builder-setup docker-builder-remove

docker-vendor:
	go mod vendor
	modvendor -copy="**/*.c **/*.h **/*.proto **/*.a" -v

docker-builder-setup: ## Setup Docker buildx for multi-arch builds
	@if ! docker buildx inspect $(DOCKER_BUILDER) > /dev/null 2>&1; then \
		echo "Creating Docker buildx builder: $(DOCKER_BUILDER)"; \
		docker buildx create --name $(DOCKER_BUILDER) --driver docker-container --bootstrap --use; \
	else \
		echo "Docker buildx builder already exists: $(DOCKER_BUILDER)"; \
		docker buildx use $(DOCKER_BUILDER); \
	fi

docker-builder-remove: ## Remove Docker buildx builder
	@if docker buildx inspect $(DOCKER_BUILDER) > /dev/null 2>&1; then \
		echo "Removing Docker buildx builder: $(DOCKER_BUILDER)"; \
		docker buildx rm $(DOCKER_BUILDER); \
	else \
		echo "Docker buildx builder does not exist: $(DOCKER_BUILDER)"; \
	fi

############################
### Single-Platform Builds ###
############################
# Builds for single architecture (current platform)
# Use FOR_DEV and FOR_TESTNET variables to control image tags
# Examples:
#   make docker-build                                    # Production: kleverapp/klever-go:v1.0.0
#   FOR_DEV=dev- make docker-build                       # Dev: kleverapp/klever-go:dev-v1.0.0
#   FOR_TESTNET=-testnet make docker-build               # Testnet: kleverapp/klever-go:v1.0.0-testnet
#   FOR_DEV=dev- FOR_TESTNET=-testnet make docker-build  # Dev testnet: kleverapp/klever-go:dev-v1.0.0-testnet

.PHONY: docker-build docker-build-validator docker-build-alpine docker-build-alpine-validator

docker-build: docker-vendor ## Build Debian-based full Docker image (production, no cache)
	@echo "Building Debian full docker image for version ${VERSION}"
	docker build --no-cache --pull --build-arg arg_version=${VERSION} \
		-t $(DOCKER_REPO):${FOR_DEV}${VERSION}${FOR_TESTNET} \
		-t $(DOCKER_REPO):${FOR_DEV}latest${FOR_TESTNET} \
		-f docker/Dockerfile .

docker-build-validator: docker-vendor ## Build Debian-based validator Docker image (production, no cache)
	@echo "Building Debian validator docker image for version ${VERSION}"
	docker build --no-cache --pull --build-arg arg_version=${VERSION} \
		-t $(DOCKER_REPO):val-${FOR_DEV}${VERSION}${FOR_TESTNET} \
		-f docker/Dockerfile.validator .

docker-build-alpine: docker-vendor ## Build Alpine-based full Docker image (production, no cache)
	@echo "Building Alpine full docker image for version ${VERSION}"
	docker build --no-cache --pull --build-arg arg_version=${VERSION} \
		-t $(DOCKER_REPO):alpine-${FOR_DEV}${VERSION}${FOR_TESTNET} \
		-f docker/Dockerfile.alpine .

docker-build-alpine-validator: docker-vendor ## Build Alpine-based validator Docker image (production, no cache)
	@echo "Building Alpine validator docker image for version ${VERSION}"
	docker build --no-cache --pull --build-arg arg_version=${VERSION} \
		-t $(DOCKER_REPO):alpine-val-${FOR_DEV}${VERSION}${FOR_TESTNET} \
		-f docker/Dockerfile.alpine.validator .

############################
### Multi-Architecture Builds
############################
# Production builds supporting both amd64 and arm64
# These automatically push to the registry

.PHONY: docker-buildx-full docker-buildx-validator docker-buildx-alpine-full docker-buildx-alpine-validator docker-buildx-all

docker-buildx-full: docker-vendor docker-builder-setup ## Build and push multi-arch Debian full image
	@echo "Building multi-arch Debian full image for version ${VERSION}"
	docker buildx build \
		--progress=plain \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg arg_version=${VERSION} \
		--tag $(DOCKER_REPO):${FOR_DEV}${VERSION}${FOR_TESTNET} \
		$(LATEST_TAG_FULL) \
		--file docker/Dockerfile \
		--push \
		.

docker-buildx-validator: docker-vendor docker-builder-setup ## Build and push multi-arch Debian validator image
	@echo "Building multi-arch Debian validator image for version ${VERSION}"
	docker buildx build \
		--progress=plain \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg arg_version=${VERSION} \
		--tag $(DOCKER_REPO):val-${FOR_DEV}${VERSION}${FOR_TESTNET} \
		$(LATEST_TAG_VALIDATOR) \
		--file docker/Dockerfile.validator \
		--push \
		.

docker-buildx-alpine-full: docker-vendor docker-builder-setup ## Build and push multi-arch Alpine full image
	@echo "Building multi-arch Alpine full image for version ${VERSION}"
	docker buildx build \
		--progress=plain \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg arg_version=${VERSION} \
		--tag $(DOCKER_REPO):alpine-${FOR_DEV}${VERSION}${FOR_TESTNET} \
		$(LATEST_TAG_ALPINE) \
		--file docker/Dockerfile.alpine \
		--push \
		.

docker-buildx-alpine-validator: docker-vendor docker-builder-setup ## Build and push multi-arch Alpine validator image
	@echo "Building multi-arch Alpine validator image for version ${VERSION}"
	docker buildx build \
		--progress=plain \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg arg_version=${VERSION} \
		--tag $(DOCKER_REPO):alpine-val-${FOR_DEV}${VERSION}${FOR_TESTNET} \
		$(LATEST_TAG_ALPINE_VALIDATOR) \
		--file docker/Dockerfile.alpine.validator \
		--push \
		.

docker-buildx-all: docker-buildx-full docker-buildx-validator docker-buildx-alpine-full docker-buildx-alpine-validator ## Build and push all multi-arch image variants

############################
###   Registry Operations ##
############################

.PHONY: docker-push

docker-push: ## Push Docker image to registry
	docker push $(DOCKER_REPO):${FOR_DEV}${VERSION}${FOR_TESTNET}
