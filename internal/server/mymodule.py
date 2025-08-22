def preprocess(tool_params: dict) -> dict:
	print("Hello from before_tool")
	for key, value in tool_params.items():
		tool_params[key] = value.upper()
	print(tool_params)
	return tool_param