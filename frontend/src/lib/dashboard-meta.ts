export const metricRangeOptions = [
  { value: "1h", label: "1小时" },
  { value: "6h", label: "6小时" },
  { value: "24h", label: "24小时" },
  { value: "7d", label: "7天" },
  { value: "all", label: "全部" },
] as const;

export function getPermissionRoleLabel(permission: number): string {
  if (permission === 3) return "Admin";
  if (permission === 2) return "ReadWrite";
  return "ReadOnly";
}
