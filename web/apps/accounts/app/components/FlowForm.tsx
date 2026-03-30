import type {
  UiContainer,
  UiNode,
  UiNodeInputAttributes,
  UiNodeTextAttributes,
  UiNodeImageAttributes,
  UiNodeScriptAttributes,
  UiNodeAnchorAttributes,
  UiNodeDivisionAttributes,
} from "@ory/kratos-client-fetch";
import { UiNodeTypeEnum } from "@ory/kratos-client-fetch";
import { FlowMessages } from "./FlowMessages";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Label } from "~/components/ui/label";
import { Separator } from "~/components/ui/separator";

interface FlowFormProps {
  ui: UiContainer;
  submitLabel?: string;
}

const GROUP_LABELS: Record<string, string> = {
  password: "Password",
  profile: "Profile",
  oidc: "Social accounts",
  code: "Verification code",
  totp: "Authenticator app",
  webauthn: "Security key",
  passkey: "Passkey",
  lookup_secret: "Recovery codes",
};

function groupLabel(group: string): string {
  return GROUP_LABELS[group] ?? (group.charAt(0).toUpperCase() + group.slice(1));
}

function nodeValue(value: unknown): string | undefined {
  if (value == null) return undefined;
  return String(value);
}

function renderNode(node: UiNode, index: number) {
  const key = `${node.group}-${index}`;

  switch (node.type) {
    case UiNodeTypeEnum.Input: {
      const attrs = node.attributes as UiNodeInputAttributes;
      const isHidden = attrs.type === "hidden";
      const isSubmit = attrs.type === "submit" || attrs.type === "button";
      const hasErrors = node.messages.length > 0;

      if (isHidden) {
        return (
          <input
            key={key}
            type="hidden"
            name={attrs.name}
            value={nodeValue(attrs.value) ?? ""}
          />
        );
      }

      if (isSubmit) {
        return (
          <Button
            key={key}
            type={attrs.type as "submit" | "button"}
            name={attrs.name}
            value={nodeValue(attrs.value) ?? ""}
            disabled={attrs.disabled}
            className="w-full"
          >
            {node.meta.label?.text ?? "Submit"}
          </Button>
        );
      }

      return (
        <div key={key} className="flex flex-col gap-1.5">
          {node.meta.label?.text && (
            <Label htmlFor={attrs.name}>{node.meta.label.text}</Label>
          )}
          <Input
            id={attrs.name}
            name={attrs.name}
            type={attrs.type as string}
            required={attrs.required}
            disabled={attrs.disabled}
            autoComplete={attrs.autocomplete as string | undefined}
            aria-invalid={hasErrors || undefined}
            defaultValue={nodeValue(attrs.value)}
          />
          <FlowMessages messages={node.messages} variant="field" />
        </div>
      );
    }

    case UiNodeTypeEnum.Text: {
      const attrs = node.attributes as UiNodeTextAttributes;
      return <div key={key} className="text-sm text-muted-foreground">{attrs.text.text}</div>;
    }

    case UiNodeTypeEnum.Img: {
      const attrs = node.attributes as UiNodeImageAttributes;
      return (
        <img
          key={key}
          src={attrs.src}
          width={attrs.width}
          height={attrs.height}
          id={attrs.id}
          alt=""
        />
      );
    }

    case UiNodeTypeEnum.A: {
      const attrs = node.attributes as UiNodeAnchorAttributes;
      return (
        <a key={key} href={attrs.href} id={attrs.id} className="text-sm underline underline-offset-4 text-muted-foreground hover:text-foreground">
          {attrs.title.text}
        </a>
      );
    }

    case UiNodeTypeEnum.Script: {
      const attrs = node.attributes as UiNodeScriptAttributes;
      return (
        <script
          key={key}
          src={attrs.src}
          integrity={attrs.integrity}
          crossOrigin={attrs.crossorigin as "" | "anonymous" | "use-credentials"}
          async={attrs.async}
          nonce={attrs.nonce}
          id={attrs.id}
          type={attrs.type}
        />
      );
    }

    case UiNodeTypeEnum.Div: {
      const attrs = node.attributes as UiNodeDivisionAttributes;
      return (
        <div
          key={key}
          id={attrs.id}
          className={attrs._class}
          data-testid={attrs.data?.["testid"]}
        />
      );
    }

    default:
      return null;
  }
}

export function FlowForm({ ui }: FlowFormProps) {
  const groups = new Map<string, UiNode[]>();

  for (const node of ui.nodes) {
    const group = groups.get(node.group) ?? [];
    group.push(node);
    groups.set(node.group, group);
  }

  const defaultNodes = groups.get("default") ?? [];
  const otherGroups = [...groups.entries()].filter(([g]) => g !== "default");

  return (
    <form method={ui.method} action={ui.action}>
      <FlowMessages messages={ui.messages} />

      <div className="flex flex-col gap-6 mt-4">
        <div className="flex flex-col gap-4">
          {defaultNodes.map((node, i) => renderNode(node, i))}
        </div>

        {otherGroups.map(([group, nodes]) => (
          <div key={group} className="flex flex-col gap-4">
            <div className="flex items-center gap-3">
              <Separator className="flex-1" />
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                {groupLabel(group)}
              </span>
              <Separator className="flex-1" />
            </div>
            {nodes.map((node, i) => renderNode(node, i))}
          </div>
        ))}
      </div>
    </form>
  );
}
