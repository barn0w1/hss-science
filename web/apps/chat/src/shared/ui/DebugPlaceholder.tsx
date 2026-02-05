interface DebugPlaceholderProps {
  label: string;
  color?: string; // Tailwind classes for bg, border, text
  className?: string; // Additional classes
}

export const DebugPlaceholder = ({ 
  label, 
  color = "bg-gray-500/20 border-gray-500/50 text-gray-700",
  className = ""
}: DebugPlaceholderProps) => (
  <div className={`w-full h-full ${color} border-2 border-dashed flex items-center justify-center text-xs font-mono font-bold uppercase tracking-widest select-none rounded-[inherit] ${className}`}>
    {label}
  </div>
);
