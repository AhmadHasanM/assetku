export type MasterKind = "locations" | "organizations" | "suppliers"

export type MasterField = "name" | "description" | "contact_name" | "phone" | "email" | "address"

export interface KindConfig {
  /** Form fields in order (name is always first). */
  fields: readonly MasterField[]
  /** Columns shown in the desktop table, besides name and asset count. */
  columns: readonly MasterField[]
}

export const KIND_CONFIG: Record<MasterKind, KindConfig> = {
  locations: { fields: ["name", "description"], columns: ["description"] },
  organizations: { fields: ["name"], columns: [] },
  suppliers: {
    fields: ["name", "contact_name", "phone", "email", "address"],
    columns: ["contact_name", "phone", "email"],
  },
}

export type MasterItem = { id: string; name: string; asset_count: number } & Partial<
  Record<Exclude<MasterField, "name">, string>
>
