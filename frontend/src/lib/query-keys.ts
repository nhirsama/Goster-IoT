export type DeviceListStatus = "all" | "authenticated" | "pending" | "refused" | "revoked";

export const queryKeys = {
  authMe: ["auth-me"] as const,
  users: ["users"] as const,
  device: (uuid: string) => ["device", uuid] as const,
  devicesByStatus: (status: DeviceListStatus) => ["devices", status] as const,
  metrics: (uuid: string, range: string) => ["metrics", uuid, range] as const,
};
