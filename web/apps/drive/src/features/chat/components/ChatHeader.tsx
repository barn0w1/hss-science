import type { ReactNode } from 'react';
import { DebugPlaceholder } from '@/shared/ui/DebugPlaceholder';

const IS_LAYOUT_DEBUG = false;

const ChatHeaderLayout = ({ children }: { children: ReactNode }) => (
  <div className="h-full w-full">
    {IS_LAYOUT_DEBUG ? (
      <DebugPlaceholder
        label="Chat Header"
        color="bg-yellow-500/20 border-yellow-500/50 text-yellow-700"
      />
    ) : (
      children
    )}
  </div>
);

export const ChatHeader = () => {
  return (
    <ChatHeaderLayout>
      <div className="relative w-full flex items-center text-surface-700 h-full">
        {/* Header content */}
      </div>
    </ChatHeaderLayout>
  );
};
