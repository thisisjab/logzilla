import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../api/apiClient";
import { type LogRecord } from "../api/models/LogRecord";
import type { PaginatedAPIResponse } from "../api/models/Response";
import type { SearchLogRequest } from "../api/schemas/SearchLog";

const searchLogs = (req: SearchLogRequest) => {
  return apiClient
    .post<PaginatedAPIResponse<LogRecord>>("/logs/search", {
      query: req.query,
    })
    .then((res) => res);
};

export const useSearchLogs = (req: SearchLogRequest) => {
  return useQuery({
    queryKey: ["logs", req.query],
    queryFn: () => searchLogs(req),
    retry: false,
  });
};
