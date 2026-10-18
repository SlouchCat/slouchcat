# @slouchcat/api-client

Typisierter Client für die Slouchcat-API, erzeugt aus `api/openapi.yaml`.

- `pnpm gen` (Teil von `task gen`) schreibt `src/generated/schema.d.ts` mit `openapi-typescript`. Die Datei
  ist eingecheckt; `task gen:check` schlägt fehl, wenn sie vom Vertrag abweicht. Nie von Hand ändern.
- `createApiClient(baseUrl = '/api/v1')` liefert einen `openapi-fetch`-Client, typisiert mit `paths`,
  z. B. `client.GET('/healthz')`.
- `isProblem(value)` prüft, ob ein Fehlerkörper ein RFC-9457-Problem (`Problem`) ist.
