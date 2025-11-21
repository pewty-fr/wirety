interface PageHeaderProps {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}

export default function PageHeader({ title, subtitle, action }: PageHeaderProps) {
  return (
    <div className="bg-gradient-to-r from-white via-gray-50 to-white dark:from-gray-800 dark:via-gray-900 dark:to-gray-800 border-b border-gray-200 dark:border-gray-700 shadow-sm">
      <div className="px-8 py-8">
        <div className="flex items-center justify-between">
          <div className="flex-1">
            <h1 className="text-3xl font-bold bg-gradient-to-r from-gray-900 to-gray-600 dark:from-white dark:to-white bg-clip-text text-transparent">{title}</h1>
            {subtitle && (
              <p className="mt-2 text-base text-gray-600 dark:text-gray-400 font-medium">{subtitle}</p>
            )}
          </div>
          {action && <div className="ml-6">{action}</div>}
        </div>
      </div>
    </div>
  );
}
