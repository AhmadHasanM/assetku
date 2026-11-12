"use client"

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { api } from "./api"
import type { Me, MeResponse } from "./types"

export const ME_QUERY_KEY = ["me"] as const

/** Current user (GET /api/auth/me). A 401 redirects to /login via api(). */
export function useMe() {
  return useQuery({
    queryKey: ME_QUERY_KEY,
    queryFn: async () => (await api<MeResponse>("/api/auth/me")).user,
    staleTime: 5 * 60_000,
  })
}

export type MePatch = Partial<Pick<Me, "name" | "position" | "locale" | "theme">>

/** PATCH /api/me; the cached Me is replaced by the response. */
export function useUpdateMe() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: async (patch: MePatch) =>
      (await api<MeResponse>("/api/me", { method: "PATCH", json: patch })).user,
    onSuccess: (me) => qc.setQueryData(ME_QUERY_KEY, me),
  })
}
