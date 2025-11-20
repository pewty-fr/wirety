import Modal from './Modal';
import type { User } from '../types';

interface UserDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  user: User | null;
}

export default function UserDetailModal({ isOpen, onClose, user }: UserDetailModalProps) {
  if (!user) return null;

  const roleColors = {
    administrator: 'bg-purple-100 text-purple-800',
    user: 'bg-primary-100 text-primary-800 dark:bg-primary-900 dark:text-primary-200',
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="User Details"
      size="lg"
    >
      <div className="space-y-6">
        {/* Header Info */}
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className="text-5xl">👤</div>
            <div>
              <h3 className="text-2xl font-bold text-gray-900 dark:text-white">{user.name}</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">ID: {user.id}</p>
              <div className="flex gap-2 mt-2">
                <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${roleColors[user.role]}`}>
                  {user.role === 'administrator' ? 'Administrator' : 'User'}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* User Info */}
        <div className="grid grid-cols-2 gap-6">
          <div>
            <label className="block text-sm font-medium text-gray-500 mb-1">Email</label>
            <p className="text-lg text-gray-900 dark:text-white">{user.email}</p>
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-500 mb-1">Role</label>
            <p className="text-lg text-gray-900 dark:text-white capitalize">{user.role}</p>
          </div>
        </div>

        {/* Authorized Networks */}
        <div>
          <label className="block text-sm font-medium text-gray-500 mb-2">
            Authorized Networks ({user.authorized_networks?.length || 0})
          </label>
          {user.authorized_networks && user.authorized_networks.length > 0 ? (
            <div className="space-y-1">
              {user.authorized_networks.map((networkId, index) => (
                <div key={index} className="bg-gray-50 dark:bg-gray-700 px-3 py-2 rounded text-sm text-gray-900 dark:text-gray-100">
                  {networkId}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-500 italic">No networks authorized</p>
          )}
        </div>

        {/* Activity Info */}
        <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
          <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Activity</h4>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-sm text-gray-600 dark:text-gray-400">Last Login</span>
              <span className="text-sm text-gray-900 dark:text-white">
                {user.last_login_at ? new Date(user.last_login_at).toLocaleString() : 'Never'}
              </span>
            </div>
          </div>
        </div>

        {/* Timestamps */}
        <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4">
          <div className="grid grid-cols-2 gap-6">
            <div>
              <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Created</label>
              <p className="text-sm text-gray-900 dark:text-white">
                {new Date(user.created_at).toLocaleString()}
              </p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">Last Updated</label>
              <p className="text-sm text-gray-900 dark:text-white">
                {new Date(user.updated_at).toLocaleString()}
              </p>
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="flex justify-end gap-3 pt-4 border-t border-gray-200">
          <button
            onClick={onClose}
            className="px-4 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 cursor-pointer transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </Modal>
  );
}
