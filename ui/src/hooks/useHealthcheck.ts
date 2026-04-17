import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../api/apiClient";
import type { HealthcheckResponse } from "../api/schemas/Healthcheck";

const fetchHealthcheck = () => {
  return apiClient.get<HealthcheckResponse>("/healthcheck");
};

export const useHealthcheck = () => {
  return useQuery({
    queryKey: ["api", "healthcheck"],
    queryFn: () => fetchHealthcheck(),
  });
};
