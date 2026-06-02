PROJECT_NAME=rangine

HELM_CHART_DIR=charts
HELM_VALUES_FILE := $(HELM_CHART_DIR)/values.yaml
HELM_CHART_NAME := $(shell awk '$$1=="name:" {print $$2; exit}' $(HELM_CHART_DIR)/Chart.yaml)
HELM_IMAGE_REPOSITORY := $(shell awk '/^image:/{flag=1; next} flag && /^[^[:space:]]/{flag=0} flag && $$1=="repository:" {print $$2; exit}' $(HELM_VALUES_FILE))
HELM_IMAGE_TAG := $(shell awk '/^image:/{flag=1; next} flag && /^[^[:space:]]/{flag=0} flag && $$1=="tag:" {print $$2; exit}' $(HELM_VALUES_FILE))
IMAGE_REPOSITORY ?= $(HELM_IMAGE_REPOSITORY)
IMAGE_TAG ?= $(HELM_IMAGE_TAG)
BETA_SUFFIX ?=
BETA_IMAGE_TAG := $(IMAGE_TAG)-$(BETA_SUFFIX)
HELM_CHART_VERSION ?= $(shell awk '$$1=="version:" {print $$2; exit}' $(HELM_CHART_DIR)/Chart.yaml)
HELM_APP_VERSION ?= $(IMAGE_TAG)
HELM_PACKAGE_IMAGE_REPOSITORY ?= $(IMAGE_REPOSITORY)
HELM_PACKAGE_IMAGE_TAG ?= $(IMAGE_TAG)
HELM_PACKAGE ?= $(HELM_CHART_DIR)/$(HELM_CHART_NAME)-$(HELM_CHART_VERSION).tgz

IMAGE_TARGET ?= $(IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: tidy dockerbuild helm-package publish beta dev test help

tidy:
	go mod tidy

dockerbuild:
	docker build -t $(IMAGE_TARGET) .

helm-package:
	@test -n "$(HELM_PACKAGE_IMAGE_REPOSITORY)" || (echo "HELM_PACKAGE_IMAGE_REPOSITORY is empty. Pass HELM_PACKAGE_IMAGE_REPOSITORY=registry.example.com/ns/image."; exit 1)
	@test -n "$(HELM_PACKAGE_IMAGE_TAG)" || (echo "HELM_PACKAGE_IMAGE_TAG is empty. Pass HELM_PACKAGE_IMAGE_TAG=vX.Y.Z."; exit 1)
	@test -n "$(HELM_CHART_VERSION)" || (echo "HELM_CHART_VERSION is empty."; exit 1)
	@test -n "$(HELM_APP_VERSION)" || (echo "HELM_APP_VERSION is empty."; exit 1)
	@tmp_dir=$$(mktemp -d); \
	cp $(HELM_CHART_DIR)/Chart.yaml $$tmp_dir/Chart.yaml; \
	cp $(HELM_CHART_DIR)/values.yaml $$tmp_dir/values.yaml; \
	test ! -f $(HELM_CHART_DIR)/Chart.lock || cp $(HELM_CHART_DIR)/Chart.lock $$tmp_dir/Chart.lock; \
	restore() { \
		cp $$tmp_dir/Chart.yaml $(HELM_CHART_DIR)/Chart.yaml; \
		cp $$tmp_dir/values.yaml $(HELM_CHART_DIR)/values.yaml; \
		if test -f $$tmp_dir/Chart.lock; then cp $$tmp_dir/Chart.lock $(HELM_CHART_DIR)/Chart.lock; else rm -f $(HELM_CHART_DIR)/Chart.lock; fi; \
		rm -rf $$tmp_dir; \
	}; \
	trap restore EXIT; \
	rm -f $(HELM_PACKAGE); \
	perl -0pi -e 's/^version:\s*.*/version: $(HELM_CHART_VERSION)/m; s/^appVersion:\s*.*/appVersion: "$(HELM_APP_VERSION)"/m' $(HELM_CHART_DIR)/Chart.yaml; \
	perl -0pi -e 's#^(\s*repository:\s*).*#$$1$(HELM_PACKAGE_IMAGE_REPOSITORY)#m' $(HELM_CHART_DIR)/values.yaml; \
	perl -0pi -e 's/^(\s*tag:\s*).*/$$1$(HELM_PACKAGE_IMAGE_TAG)/m' $(HELM_CHART_DIR)/values.yaml; \
	helm package $(HELM_CHART_DIR) --destination $(HELM_CHART_DIR)

publish: dockerbuild helm-package
	@test -n "$(IMAGE_TAG)" || (echo "IMAGE_TAG is empty. Run from a git tag or pass IMAGE_TAG=vX.Y.Z."; exit 1)
	docker push $(IMAGE_TARGET)

beta:
	@if [ -z "$(BETA_SUFFIX)" ]; then \
		echo "BETA_SUFFIX is required, for example: make beta BETA_SUFFIX=beta1"; \
		exit 1; \
	fi
	$(MAKE) publish IMAGE_TAG=$(BETA_IMAGE_TAG) HELM_APP_VERSION=$(BETA_IMAGE_TAG)

dev:
	go run *.go server:start

test:
	go test -v ./...

help:
	@echo "make dockerbuild - 使用 Dockerfile 构建镜像"
	@echo "make helm-package - 使用当前 Helm chart 打包，并临时替换镜像仓库、tag、chart version"
	@echo "make publish - 构建镜像、打包 Helm，并推送镜像"
	@echo "make beta BETA_SUFFIX=beta1 - 使用当前镜像 tag 加手动后缀发布 beta，例如 $(IMAGE_TAG)-beta1"
	@echo "make dev - 本地运行 $(PROJECT_NAME)"
	@echo "make test - 运行 Go 测试"
