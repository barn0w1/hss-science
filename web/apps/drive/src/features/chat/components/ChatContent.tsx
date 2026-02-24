import type { ReactNode } from 'react';
import { DebugPlaceholder } from '@/shared/ui/DebugPlaceholder';

const IS_LAYOUT_DEBUG = false;

const ChatContentLayout = ({ children }: { children: ReactNode }) => (
    <div className="h-full flex flex-col pr-[var(--layout-padding)] pb-[var(--layout-padding)]">
        {IS_LAYOUT_DEBUG ? (
            <DebugPlaceholder
                label="Chat Content"
                color="bg-blue-500/20 border-blue-500/50 text-blue-700"
                className="rounded-[var(--radius-panel)]"
            />
        ) : (
            children
        )}
    </div>
);

export const ChatContent = () => {
    return (
        <ChatContentLayout>
            <div className="layout-content-body h-full flex flex-col rounded-[var(--radius-panel)] overflow-hidden">
                <div className="border-b border-surface-100 px-10 py-6">
                    <div className="flex items-center justify-between">
                        <div>
                            <div className="h-6 w-40 rounded-full bg-surface-100 mt-2" />
                        </div>
                        <div className="h-9 w-24 rounded-[var(--radius-pill)] bg-surface-100" />
                    </div>
                </div>

                <div className="flex-1 overflow-y-auto px-10 py-10 space-y-6">
                    {Array.from({ length: 4 }).map((_, index) => (
                        <div key={index} className="space-y-2">
                            <div className="h-4 w-40 rounded-full bg-surface-100" />
                            <div className="h-3 w-3/5 rounded-full bg-surface-100" />
                            <div className="h-3 w-2/5 rounded-full bg-surface-100" />
                        </div>
                    ))}
                </div>
                <div className="border-t border-surface-100 px-8 py-6">
                    <div className="h-12 w-full rounded-[var(--radius-pill)] bg-surface-50 border border-surface-100" />
                </div>
            </div>
        </ChatContentLayout>
    );
};