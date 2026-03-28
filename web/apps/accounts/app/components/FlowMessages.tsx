import type { UiText } from "@ory/kratos-client-fetch";

interface FlowMessagesProps {
  messages?: UiText[];
}

export function FlowMessages({ messages }: FlowMessagesProps) {
  if (!messages || messages.length === 0) {
    return null;
  }

  return (
    <div>
      {messages.map((message) => {
        const className =
          message.type === "error"
            ? "text-red-600"
            : message.type === "success"
              ? "text-green-600"
              : "text-gray-600";
        return (
          <p key={message.id} className={className}>
            {message.text}
          </p>
        );
      })}
    </div>
  );
}
