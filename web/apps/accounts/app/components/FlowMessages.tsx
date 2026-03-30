import type { UiText } from "@ory/kratos-client-fetch";
import { Alert, AlertDescription } from "~/components/ui/alert";

interface FlowMessagesProps {
  messages?: UiText[];
  variant?: "alert" | "field";
}

type AlertVariant = "default" | "destructive" | "success";

function alertVariantFor(type: string): AlertVariant {
  if (type === "error") return "destructive";
  if (type === "success") return "success";
  return "default";
}

export function FlowMessages({ messages, variant = "alert" }: FlowMessagesProps) {
  if (!messages || messages.length === 0) {
    return null;
  }

  if (variant === "field") {
    return (
      <div className="flex flex-col gap-1">
        {messages.map((message) => {
          const className =
            message.type === "error"
              ? "text-sm text-destructive"
              : message.type === "success"
                ? "text-sm text-green-700 dark:text-green-400"
                : "text-sm text-muted-foreground";
          return (
            <p key={message.id} className={className}>
              {message.text}
            </p>
          );
        })}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      {messages.map((message) => (
        <Alert key={message.id} variant={alertVariantFor(message.type)}>
          <AlertDescription>{message.text}</AlertDescription>
        </Alert>
      ))}
    </div>
  );
}
