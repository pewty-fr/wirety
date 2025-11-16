import React, { useState } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, HelperText } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { TextInput, FormButton } from '../../components/FormComponents';
import { validateEndpoint, validatePort } from '../../utils/validation';

export const PeerAddJumpScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId } = route.params as { networkId: string };
  const [name, setName] = useState('');
  const [endpoint, setEndpoint] = useState('');
  const [listenPort, setListenPort] = useState('51820');
  const [natInterface, setNatInterface] = useState('eth0');
  const [additionalIPs, setAdditionalIPs] = useState('');
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<{ [key: string]: string }>({});

  const validate = () => {
    const newErrors: { [key: string]: string } = {};
    
    if (!name.trim()) newErrors.name = 'Name is required';
    if (!endpoint.trim()) {
      newErrors.endpoint = 'Endpoint is required for jump server';
    } else if (!validateEndpoint(endpoint)) {
      newErrors.endpoint = 'Invalid endpoint format (expected IP:PORT)';
    }
    const port = parseInt(listenPort, 10);
    if (!validatePort(port)) {
      newErrors.listenPort = 'Invalid port number';
    }
    if (!natInterface.trim()) {
      newErrors.natInterface = 'NAT interface is required';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async () => {
    if (!validate()) return;

    setLoading(true);
    try {
      const additional_allowed_ips = additionalIPs
        .split(',')
        .map((ip) => ip.trim())
        .filter((ip) => ip);

      await api.createPeer(networkId, {
        name,
        endpoint,
        listen_port: parseInt(listenPort, 10),
        is_jump: true,
        jump_nat_interface: natInterface,
        is_isolated: false,
        full_encapsulation: false,
        additional_allowed_ips: additional_allowed_ips.length > 0 ? additional_allowed_ips : undefined,
      });
      // Go back twice: past choice screen to peer list
      navigation.goBack();
      navigation.goBack();
    } catch (error) {
      console.error('Failed to create jump server:', error);
      setErrors({ submit: 'Failed to create jump server' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView style={styles.container}>
      <View style={styles.form}>
        <Title>Add Jump Server</Title>

        <TextInput
          label="Name"
          value={name}
          onChangeText={setName}
          placeholder="Jump Server 1"
          error={errors.name}
        />
        {errors.name && <HelperText type="error">{errors.name}</HelperText>}

        <TextInput
          label="Endpoint"
          value={endpoint}
          onChangeText={setEndpoint}
          placeholder="1.2.3.4:51820"
          error={errors.endpoint}
        />
        {errors.endpoint && <HelperText type="error">{errors.endpoint}</HelperText>}

        <TextInput
          label="Listen Port"
          value={listenPort}
          onChangeText={setListenPort}
          placeholder="51820"
          keyboardType="numeric"
          error={errors.listenPort}
        />
        {errors.listenPort && <HelperText type="error">{errors.listenPort}</HelperText>}

        <TextInput
          label="NAT Interface"
          value={natInterface}
          onChangeText={setNatInterface}
          placeholder="eth0"
          error={errors.natInterface}
        />
        {errors.natInterface && <HelperText type="error">{errors.natInterface}</HelperText>}

        <TextInput
          label="Additional Allowed IPs (optional)"
          value={additionalIPs}
          onChangeText={setAdditionalIPs}
          placeholder="192.168.1.0/24, 10.0.0.0/8"
          multiline
        />
        <HelperText type="info">
          Comma-separated list of additional IP ranges this peer can route
        </HelperText>

        {errors.submit && <HelperText type="error">{errors.submit}</HelperText>}

        <FormButton title="Create Jump Server" onPress={handleSubmit} loading={loading} />
        <FormButton
          title="Cancel"
          onPress={() => navigation.goBack()}
          mode="outlined"
        />
      </View>
    </ScrollView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#fff',
  },
  form: {
    padding: 16,
  },
});
