// The GET /api/packs wire types. They mirror the Go response in
// internal/daemon/packs_http.go field for field: the installed packs with their
// versioned bundle URLs and raw block schemas, plus the dropped candidates. The
// SPA never validates a pack payload against these schemas (the REST edge does);
// they ride along for future tooling and for enumerating block types.

// PackBlockInfo is one block type a pack declares. `type` is the full dotted
// wire type (`<pack>.<name>`); `interactive` is true when the manifest declared
// an interaction schema. `schema`/`interaction` are the raw JSON Schema bodies,
// opaque to the host.
export interface PackBlockInfo {
  type: string;
  interactive: boolean;
  schema: unknown;
  interaction?: unknown;
}

// PackInfo is one installed pack. `bundle` and `styles` are ready-to-import URLs
// under /packs/<name>/... already cache-busted with the version query.
export interface PackInfo {
  name: string;
  version: string;
  description: string;
  bundle: string;
  styles?: string;
  blocks: PackBlockInfo[];
}

// DroppedInfo is a candidate the daemon refused to load: its directory base name
// and a human reason. Surfaced for diagnostics only.
export interface DroppedInfo {
  dir: string;
  reason: string;
}

export interface PacksResponse {
  hostApi: number;
  packs: PackInfo[];
  dropped: DroppedInfo[];
}
