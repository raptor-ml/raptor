BASEURL = https://raw.githubusercontent.com/aws/sagemaker-python-sdk/master/src/sagemaker/image_uri_config/
FRAMEWORKS = huggingface pytorch sklearn tensorflow xgboost

.PHONY: get-configs
get-configs:
	@echo "Downloading SageMaker framework container image URI configs..."
	@for framework in $(FRAMEWORKS); do \
  		echo "  → Downloading $$framework config..."; \
		curl -s $(BASEURL)$$framework.json > image_uri_config/$$framework.json; \
	done
