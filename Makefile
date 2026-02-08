.PHONY: proto plugin

PLUGIN_VERSION ?= v0.1.0
PLUGIN_IMAGE = plugins.buf.build/linkbreakers-com/grpc-mcp-gateway:$(PLUGIN_VERSION)

proto:
	buf generate

plugin:
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(PLUGIN_IMAGE) \
		--push .
	buf registry plugin push \
		buf.build/linkbreakers-com/grpc-mcp-gateway \
		--version $(PLUGIN_VERSION) \
		--image $(PLUGIN_IMAGE) \
		--visibility public
