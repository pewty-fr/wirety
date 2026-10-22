import { ArrowRightStartOnRectangleIcon } from '@heroicons/react/20/solid';
import { Button, Card, CardHeader, Page, PageHeader } from '../../components/ui';
import { useAuth } from '../../contexts/AuthContext';
import { AccountDetails } from './AccountDetails';
import { ApiTokens } from './ApiTokens';

export default function ProfilePage() {
  const { logout } = useAuth();

  return (
    <Page>
      <PageHeader
        title="Profile"
        description="Your account and the API tokens that act on your behalf."
        actions={
          <Button variant="secondary" icon={<ArrowRightStartOnRectangleIcon className="size-4" />} onClick={() => logout()}>
            Sign out
          </Button>
        }
      />
      <div className="max-w-3xl space-y-6">
        <Card>
          <CardHeader title="Account" description="Your role and networks are managed by administrators." />
          <div className="px-5 py-4">
            <AccountDetails />
          </div>
        </Card>
        <ApiTokens />
      </div>
    </Page>
  );
}
