# Generated Python PDP client

`v1/policy_pb2.py` and `v1/policy_pb2_grpc.py` are generated from
`proto/v1/policy.proto`. Do not edit them manually.

Regenerate both Go and Python clients from the repository root:

```bash
make generate-proto
```

Add `clients/python` to `PYTHONPATH`, then import the client as
`from v1 import policy_pb2, policy_pb2_grpc`.

Install the matching generated-code runtimes first:

```bash
python -m pip install -r clients/python/requirements.txt
```
