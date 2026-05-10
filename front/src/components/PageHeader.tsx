interface PageHeaderProps {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}

export default function PageHeader({ title, subtitle, action }: PageHeaderProps) {
  return (
    <div className="bg-gradient-to-r from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-900 dark:to-gray-800 border-b border-gray-200 dark:border-gray-700 shadow-sm">
      {/* Mobile: leave room on the left (pl-16) for the hamburger button which
          floats at top-4 left-4 with z-50; reduce vertical padding too. */}
      <div className="pl-16 pr-4 py-5 sm:px-8 sm:py-8">
        <div className="flex items-center justify-between gap-3">
          <div className="flex-1 min-w-0">
            <h1 className="text-2xl sm:text-3xl font-bold bg-gradient-to-r from-gray-900 to-gray-600 dark:from-white dark:to-white bg-clip-text text-transparent truncate">{title}</h1>
            {subtitle && (
              <p className="mt-1 sm:mt-2 text-sm sm:text-base text-gray-600 dark:text-gray-400 font-medium">{subtitle}</p>
            )}
          </div>
          {action && <div className="ml-2 sm:ml-6 shrink-0">{action}</div>}
        </div>
      </div>
    </div>
  );
}
