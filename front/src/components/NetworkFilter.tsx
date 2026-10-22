import type { Network } from '../types';
import { networkOption } from '../utils/networks';
import { Combobox } from './ui';


/** Filters a list page by network; '' means every network. */
export default function NetworkFilter({
  networks,
  value,
  onChange,
}: {
  networks: Network[];
  value: string;
  onChange: (networkId: string) => void;
}) {
  return (
    <Combobox
      label="Network"
      value={value}
      onChange={onChange}
      options={[{ value: '', label: 'All networks' }, ...networks.map(networkOption)]}
      placeholder="All networks"
      className="w-full sm:w-72"
    />
  );
}
