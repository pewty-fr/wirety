import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import Heading from '@theme/Heading';

import styles from '@site/src/pages/index.module.css';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <img
          src="/img/logo.svg"
          alt="Logo Wirety"
          style={{width: '200px', marginBottom: '2rem'}}
        />
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">Maillage WireGuard moderne, sécurité dynamique & architecture Zero‑Trust.</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/intro">
            Commencer 🚀
          </Link>
          <Link
            className="button button--outline button--secondary button--lg"
            to="https://github.com/pewty-fr/wirety"
            style={{marginLeft: '1rem'}}>
            Voir sur GitHub
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={`${siteConfig.title} - Maillage WireGuard`}
      description="Wirety : réseau mesh WireGuard moderne avec sécurité dynamique et Zero‑Trust">
      <HomepageHeader />
      <main>
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
