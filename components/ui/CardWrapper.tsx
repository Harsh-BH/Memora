import { ReactNode } from "react";

type CardWrapperProps = {
  children: ReactNode;
  className?: string;
};

export function CardWrapper({ children, className = "" }: CardWrapperProps) {
  return (
    <div
      className={`rounded-3xl border border-gray-200 bg-white p-6 shadow-sm sm:p-8 ${className}`.trim()}
    >
      {children}
    </div>
  );
}
