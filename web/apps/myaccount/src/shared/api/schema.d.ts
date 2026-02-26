export interface paths {
  "/auth/login": {
    get: {
      parameters: {
        query?: {
          return_to?: string;
        };
      };
      responses: {
        302: {
          content: never;
        };
      };
    };
  };
  "/auth/callback": {
    get: {
      parameters: {
        query: {
          code: string;
          state: string;
        };
      };
      responses: {
        302: {
          content: never;
        };
        400: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/auth/logout": {
    post: {
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["MessageResponse"];
          };
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/auth/session": {
    get: {
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["SessionInfo"];
          };
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/api/v1/profile": {
    get: {
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["Profile"];
          };
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
    patch: {
      requestBody: {
        content: {
          "application/json": components["schemas"]["UpdateProfileRequest"];
        };
      };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["Profile"];
          };
        };
        400: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/api/v1/linked-accounts": {
    get: {
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["LinkedAccountList"];
          };
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/api/v1/linked-accounts/{id}": {
    delete: {
      parameters: {
        path: {
          id: string;
        };
      };
      responses: {
        204: {
          content: never;
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
        409: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/api/v1/sessions": {
    get: {
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["SessionList"];
          };
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/api/v1/sessions/{id}": {
    delete: {
      parameters: {
        path: {
          id: string;
        };
      };
      responses: {
        204: {
          content: never;
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
        404: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
  "/api/v1/account": {
    delete: {
      responses: {
        204: {
          content: never;
        };
        401: {
          content: {
            "application/json": components["schemas"]["Error"];
          };
        };
      };
    };
  };
}

export interface components {
  schemas: {
    Profile: {
      user_id: string;
      email: string;
      email_verified: boolean;
      given_name: string;
      family_name: string;
      picture?: string;
      locale: string;
      created_at: string;
      updated_at: string;
    };
    UpdateProfileRequest: {
      given_name?: string;
      family_name?: string;
      picture?: string;
      locale?: string;
    };
    LinkedAccount: {
      id: string;
      provider: string;
      external_sub: string;
      linked_at: string;
    };
    LinkedAccountList: {
      linked_accounts: components["schemas"]["LinkedAccount"][];
    };
    Session: {
      session_id: string;
      client_id: string;
      scopes: string[];
      auth_time: string;
      expires_at: string;
      created_at: string;
    };
    SessionList: {
      sessions: components["schemas"]["Session"][];
    };
    SessionInfo: {
      authenticated: boolean;
      user_id: string;
      email: string;
      given_name: string;
      family_name: string;
      picture?: string;
    };
    MessageResponse: {
      message: string;
    };
    Error: {
      code: string;
      message: string;
    };
  };
  responses: never;
  parameters: never;
  requestBodies: never;
  headers: never;
  pathItems: never;
}

export type $defs = Record<string, never>;
export type operations = Record<string, never>;
