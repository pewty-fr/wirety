import { ArrowRightStartOnRectangleIcon } from '@heroicons/react/20/solid';
import { useAuth } from '../contexts/AuthContext';
import { AccountDetails } from '../pages/profile/AccountDetails';
import { ApiTokens } from '../pages/profile/ApiTokens';
import { Button, Modal, ModalBody, ModalFooter } from './ui';

interface ProfileModalProps {
  isOpen: boolean;
  onClose: () => void;
}

/** The signed-in user's profile, opened from the sidebar: the same content as the profile page. */
export default function ProfileModal({ isOpen, onClose }: ProfileModalProps) {
  const { user, logout } = useAuth();
  if (!user) return null;

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Profile" description="Your account and the API tokens that act on your behalf." size="lg">
      <ModalBody className="space-y-6">
        <AccountDetails />
        <div className="border-t border-line pt-5">
          <ApiTokens compact />
        </div>
      </ModalBody>
      <ModalFooter>
        <Button
          variant="ghost"
          className="sm:mr-auto"
          icon={<ArrowRightStartOnRectangleIcon className="size-4" />}
          onClick={() => {
            onClose();
            logout();
          }}
        >
          Sign out
        </Button>
        <Button variant="secondary" onClick={onClose}>
          Done
        </Button>
      </ModalFooter>
    </Modal>
  );
}
