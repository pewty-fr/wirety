import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  Svg: React.ComponentType<React.ComponentProps<'svg'>>;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Secure WireGuard Mesh',
    Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>
        Build and operate encrypted overlay networks in minutes. Wirety automates peer
        provisioning, keys, ACLs and isolation so you can focus on your infrastructure—not
        manual tunnel management.
      </>
    ),
  },
  {
    title: 'Real‑Time Topology & Insight',
    Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>
        Always know how peers connect. Interactive topology highlights jump servers, direct
        paths, blocked edges and security incidents, giving actionable observability across
        environments.
      </>
    ),
  },
  {
    title: 'Zero‑Touch Peer Lifecycle',
    Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>
        Agents or static configs—your choice. Issue tokens, rotate credentials, enforce full
        tunnel or isolation policies, and adjust allowed networks centrally with no manual
        reconfiguration on hosts.
      </>
    ),
  },
  {
    title: 'Integrated Security Response',
    Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    description: (
      <>
        Detect conflicting sessions, endpoint churn and suspicious activity. Wirety raises
        incidents and can automatically block affected peers, reducing dwell time and
        tightening your blast radius.
      </>
    ),
  },
  {
    title: 'Capacity & Governance',
    Svg: require('@site/static/img/undraw_docusaurus_mountain.svg').default,
    // Svg: require('@site/static/img/undraw_capacity.svg').default,
    description: (
      <>
        Track peer counts, CIDR capacity and remaining slots per network. Role‑based access
        controls and default authorization templates keep growth predictable and compliant.
      </>
    ),
  },
];

function Feature({title, Svg, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4')}>
      <div className="text--center">
        <Svg className={styles.featureSvg} role="img" />
      </div>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
