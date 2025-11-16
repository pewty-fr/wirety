import React, { useState } from 'react';
import { View, ScrollView, StyleSheet } from 'react-native';
import { Title, HelperText, Switch, Text } from 'react-native-paper';
import { useNavigation, useRoute } from '@react-navigation/native';
import api from '../../services/api';
import { TextInput, FormButton } from '../../components/FormComponents';
import { validateEndpoint } from '../../utils/validation';

export const PeerAddRegularScreen = () => {
  const navigation = useNavigation();
  const route = useRoute();
  const { networkId } = route.params as { networkId: string };
  const [name, setName] = useState('');
  const [endpoint, setEndpoint] = useState('');
  const [isIsolated, setIsIsolated] = useState(false);
  const [fullEncapsulation, setFullEncapsulation] = useState(false);
  const [additionalIPs, setAdditionalIPs] = useState('');
  const [loading, setLoading] = useState(false);
  const [errors, setErrors] = useState<{ [key: string]: string }>({});

  const validate = () => {
    const newErrors: { [key: string]: string } = {};
    
    if (!name.trim()) newErrors.name = 'Name is required';
    if (endpoint && !validateEndpoint(endpoint)) {
      newErrors.endpoint = 'Invalid endpoint format (expected IP:PORT)';
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
        endpoint: endpoint || undefined,
        is_jump: false,
        is_isolated: isIsolated,
        full_encapsulation: fullEncapsulation,
        additional_allowed_ips: additional_allowed_ips.length > 0 ? additional_allowed_ips : undefined,
      });
      navigation.goBack();
    } catch (error) {
      console.error('Failed to create peer:', error);
      setErrors({ submit: 'Failed to create peer' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView style={styles.container}>
      <View style={styles.form}>
        <Title>Add Regular Peer</Title>

        <TextInput
          label="Name"
          value={name}
          onChangeText={setName}
          placeholder="My Laptop"
          error={errors.name}
        />
        {errors.name && <HelperText type="error">{errors.name}</HelperText>}

        <TextInput
          label="Endpoint (optional)"
          value={endpoint}
          onChangeText={setEndpoint}
          placeholder="1.2.3.4:51820"
          error={errors.endpoint}
        />
        {errors.endpoint && <HelperText type="error">{errors.endpoint}</HelperText>}

        <View style={styles.switchRow}>
          <Text>Isolated</Text>
          <Switch value={isIsolated} onValueChange={setIsIsolated} />
        </View>
        <HelperText type="info">
          Isolated peers cannot connect to other regular peers
        </HelperText>

        <View style={styles.switchRow}>
          <Text>Full Encapsulation</Text>
          <Switch value={fullEncapsulation} onValueChange={setFullEncapsulation} />
        </View>
        <HelperText type="info">
          Route all traffic (0.0.0.0/0) through jump server
        </HelperText>

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

        <FormButton title="Create Peer" onPress={handleSubmit} loading={loading} />
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
  switchRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginVertical: 8,
  },
});
