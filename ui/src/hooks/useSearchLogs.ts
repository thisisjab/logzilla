import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../api/apiClient";
import { type LogRecord } from "../api/models/LogRecord";
import type { PaginatedAPIResponse } from "../api/models/Response";

const searchLogs = (query: string) => {
  return apiClient
    .post<PaginatedAPIResponse<LogRecord>>("/logs/search", {
      query: query,
    })
    .then((res) => res);
};

export const useSearchLogs = (query: string) => {
  return useQuery({
    queryKey: ["logs", query],
    queryFn: () => searchLogs(query),
  });
};
