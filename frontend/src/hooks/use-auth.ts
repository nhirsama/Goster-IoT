import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useRouter } from "next/navigation";
import { queryKeys } from "@/lib/query-keys";

type AuthSession = components["schemas"]["AuthSession"];

export function useAuth() {
  const queryClient = useQueryClient();
  const router = useRouter();

  const { data: user, isLoading, refetch } = useQuery<AuthSession>({
    queryKey: queryKeys.authMe,
    queryFn: () => api.get("/api/v1/auth/me"),
    retry: false,
    staleTime: 5 * 60 * 1000,
  });

  const logoutMutation = useMutation({
    mutationFn: () => api.post("/api/v1/auth/logout"),
    onSuccess: () => {
      queryClient.setQueryData(queryKeys.authMe, null);
      router.push("/login");
    },
  });

  return {
    user,
    isLoading,
    isAuthenticated: !!user && user.authenticated !== false,
    logout: logoutMutation.mutate,
    refetch,
  };
}
