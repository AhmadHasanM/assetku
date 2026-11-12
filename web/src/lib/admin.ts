"use client"

import { useQuery } from "@tanstack/react-query"
import { api } from "./api"

/** Super admin, as returned by /api/admin/auth/* wrapped in {"admin": AdminMe}. */
export interface AdminMe {
  id: string
  name: string
  email: string
}

export interface AdminMeResponse {
  admin: AdminMe
}

/** One row of GET /api/admin/tenants. */
export interface AdminTenant {
  id: string
  name: string
  active: boolean
  user_count: number
  asset_count: number
  created_at: string
}

/** One row of GET /api/admin/tenants/{id}/users. */
export interface AdminUser {
  id: string
  name: string
  email: string
  position: string
  active: boolean
}

export const ADMIN_ME_QUERY_KEY = ["admin", "me"] as const
export const ADMIN_TENANTS_QUERY_KEY = ["admin", "tenants"] as const
export const adminUsersQueryKey = (tenantId: string) => ["admin", "tenants", tenantId, "users"] as const

/** Current super admin. A 401 redirects to /admin/login via api(). */
export function useAdminMe() {
  return useQuery({
    queryKey: ADMIN_ME_QUERY_KEY,
    queryFn: async () => (await api<AdminMeResponse>("/api/admin/auth/me")).admin,
    staleTime: 5 * 60_000,
  })
}

export function useAdminTenants() {
  return useQuery({
    queryKey: ADMIN_TENANTS_QUERY_KEY,
    queryFn: () => api<AdminTenant[]>("/api/admin/tenants"),
  })
}

export function useAdminUsers(tenantId: string) {
  return useQuery({
    queryKey: adminUsersQueryKey(tenantId),
    queryFn: () => api<AdminUser[]>(`/api/admin/tenants/${tenantId}/users`),
  })
}

/** Password rules mirrored from the API (auth.PasswordProblem): 8+ characters, at most 72 bytes. */
export function passwordProblem(pw: string): "too_short" | "too_long" | null {
  if ([...pw].length < 8) return "too_short"
  if (new TextEncoder().encode(pw).length > 72) return "too_long"
  return null
}
