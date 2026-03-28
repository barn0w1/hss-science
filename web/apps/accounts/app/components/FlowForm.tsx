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

interface FlowFormProps {
  ui: UiContainer;
  submitLabel?: string;
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

      return (
        <div key={key}>
          {!isHidden && (
            <label>
              {node.meta.label?.text}
            </label>
          )}
          <input
            name={attrs.name}
            type={attrs.type as string}
            required={attrs.required}
            disabled={attrs.disabled}
            autoComplete={attrs.autocomplete as string | undefined}
            {...(isHidden || isSubmit
              ? { value: nodeValue(attrs.value) ?? "" }
              : { defaultValue: nodeValue(attrs.value) })}
          />
          <FlowMessages messages={node.messages} />
        </div>
      );
    }

    case UiNodeTypeEnum.Text: {
      const attrs = node.attributes as UiNodeTextAttributes;
      return <div key={key}>{attrs.text.text}</div>;
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
        <a key={key} href={attrs.href} id={attrs.id}>
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

      <div>
        {defaultNodes.map((node, i) => renderNode(node, i))}
      </div>

      {otherGroups.map(([group, nodes]) => (
        <section key={group}>
          <h2>{group}</h2>
          {nodes.map((node, i) => renderNode(node, i))}
        </section>
      ))}
    </form>
  );
}
