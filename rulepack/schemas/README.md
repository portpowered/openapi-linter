# OpenAPI structural schemas

These dated schemas are vendored from the OpenAPI Initiative for offline validation:

- [OpenAPI 3.0 schema](https://spec.openapis.org/oas/3.0/schema/2024-10-18)
- [OpenAPI 3.1 schema](https://spec.openapis.org/oas/3.1/schema/2025-09-15)

The 3.1 schema validates document structure without Schema Object validation, as stated in its description. Example validation separately compiles effective Schema Objects with a JSON Schema validator. Structural checking does not prove semantic correctness or that every schema accepts a value.

Upstream specification and schema licensing: [OpenAPI Initiative license](https://github.com/OAI/OpenAPI-Specification/blob/main/LICENSE).
