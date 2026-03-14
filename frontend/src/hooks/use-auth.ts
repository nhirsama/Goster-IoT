import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useRouter } from "next/navigation";

type AuthSession = components["schemas"]["AuthSession"];

export function useAuth() {
  const queryClient = useQueryClient();
  const router = useRouter();

  const { data: user, isLoading, refetch } = useQuery<AuthSession>({
    queryKey: ["auth-me"],
    queryFn: () => api.get("/api/v1/auth/me"),
    retry: false,
    staleTime: 5 * 60 * 1000,
  });

  const logoutMutation = useMutation({
    mutationFn: () => api.post("/api/v1/auth/logout"),
    onSuccess: () => {
      queryClient.setQueryData(["auth-me"], null);
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
