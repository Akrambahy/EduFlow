import { forwardRef } from "react";
import { motion } from "motion/react";
import { Loader2 } from "lucide-react";

const variantClasses = {
  primary:
    "bg-teal-600 text-white hover:bg-teal-700 focus-visible:ring-teal-500 shadow-sm",
  secondary:
    "border border-teal-600 text-teal-600 hover:bg-teal-50 focus-visible:ring-teal-500",
  ghost: "text-slate-600 hover:bg-slate-100 focus-visible:ring-slate-400",
  danger:
    "bg-red-600 text-white hover:bg-red-700 focus-visible:ring-red-500 shadow-sm",
};

const sizeClasses = {
  sm: "px-3 py-1.5 text-sm gap-1.5",
  md: "px-4 py-2 text-sm gap-2",
  lg: "px-6 py-3 text-base gap-2.5",
};

const iconSizeMap = {
  sm: 14,
  md: 16,
  lg: 18,
};

/**
 * Reusable Button component with variants, sizes, loading and disabled states.
 *
 * @example
 * <Button variant="primary" size="md" loading={false}>Click Me</Button>
 */
export const Button = forwardRef(function Button(
  {
    variant = "primary",
    size = "md",
    loading = false,
    disabled = false,
    children,
    className = "",
    type = "button",
    icon: Icon,
    ...props
  },
  ref,
) {
  const isDisabled = disabled || loading;

  return (
    <motion.button
      ref={ref}
      type={type}
      disabled={isDisabled}
      whileHover={isDisabled ? undefined : { scale: 1.02 }}
      whileTap={isDisabled ? undefined : { scale: 0.97 }}
      transition={{ type: "spring", stiffness: 400, damping: 20 }}
      className={[
        "inline-flex items-center justify-center font-medium rounded-lg",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2",
        "transition-colors duration-150",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        variantClasses[variant],
        sizeClasses[size],
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      {...props}
    >
      {loading ? (
        <Loader2
          size={iconSizeMap[size]}
          className="animate-spin"
          aria-hidden="true"
        />
      ) : Icon ? (
        <Icon size={iconSizeMap[size]} aria-hidden="true" />
      ) : null}
      {children}
    </motion.button>
  );
});
