import type { Client } from "openapi-fetch";

import type { paths as DrivePaths } from "../generated/drive";
import { createTypedClient, type CreateClientOptions } from "../client";

export type { DrivePaths };
export type DriveClient = Client<DrivePaths>;

export const createDriveClient = (
  baseUrl: string,
  init?: CreateClientOptions
): DriveClient => {
  return createTypedClient<DrivePaths>(baseUrl, init);
};
