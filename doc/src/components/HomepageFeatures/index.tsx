import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import Translate, {translate} from '@docusaurus/Translate';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faNetworkWired, faCogs, faShieldAlt, faChartLine } from '@fortawesome/free-solid-svg-icons';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  icon: any; // FontAwesome IconDefinition
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: translate({id: 'feature.secureMesh.title', message: 'Secure WireGuard Mesh'}),
    icon: faNetworkWired,
    description: (
      <Translate id="feature.secureMesh.desc">
        Build and operate encrypted overlay networks in minutes. Wirety automates peer provisioning, keys, ACLs and isolation so you can focus on your infrastructure—not manual tunnel management.
      </Translate>
    ),
  },
  {
    title: translate({id: 'feature.lifecycle.title', message: 'Zero-Touch Peer Lifecycle'}),
    icon: faCogs,
    description: (
      <Translate id="feature.lifecycle.desc">
        Agents or static configs—your choice. Issue tokens, rotate credentials, enforce full tunnel or isolation policies, and adjust allowed networks centrally with no manual reconfiguration on hosts.
      </Translate>
    ),
  },
  {
    title: translate({id: 'feature.security.title', message: 'Integrated Security Response'}),
    icon: faShieldAlt,
    description: (
      <Translate id="feature.security.desc">
        Detect conflicting sessions, endpoint churn and suspicious activity. Wirety raises incidents and can automatically block affected peers, reducing dwell time and tightening your blast radius.
      </Translate>
    ),
  },
  {
    title: translate({id: 'feature.capacity.title', message: 'Capacity & Governance'}),
    icon: faChartLine,
    description: (
      <Translate id="feature.capacity.desc">
        Track peer counts, CIDR capacity and remaining slots per network. Role-based access controls and default authorization templates keep growth predictable and compliant.
      </Translate>
    ),
  },
];

function Feature({title, icon, description}: FeatureItem) {
  return (
    <div className={clsx('col col--3', styles.col)}>
      <div className={styles.featureCard}>
        <div className={styles.iconWrap}>
          <FontAwesomeIcon icon={icon} className={styles.icon} />
        </div>
        <Heading as="h3" className={styles.title}>{title}</Heading>
        <p className={styles.desc}>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className={clsx('row', styles.row)}>
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
