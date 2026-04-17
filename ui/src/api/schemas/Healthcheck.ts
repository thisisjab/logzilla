import type { APIResponse } from "../models/Response";

export interface HealthcheckResponse extends APIResponse<null> {
  metadata: {
    uptime: string;
  };
}
